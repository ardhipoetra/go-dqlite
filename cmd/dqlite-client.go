package main

import (
	"context"
	"database/sql"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/canonical/go-dqlite/client"
	"github.com/canonical/go-dqlite/driver"
	"github.com/canonical/go-dqlite/protocol"
	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"
)

var mycli1 *client.Client
var mycli2 *client.Client
var mycli3 *client.Client

const (
	// schema = "CREATE TABLE IF NOT EXISTS model (key TEXT, value TEXT, UNIQUE(key))"
	query  = "SELECT value FROM model WHERE key = ?"
	update = "INSERT OR REPLACE INTO model(key, value) VALUES(?, ?)"

	client1 = "172.17.0.3:5001"
	client2 = "172.17.0.3:5002"
	client3 = "172.17.0.3:5003"
)

func connectClients() {
	// connect each client with dqlite instance
	mycli1, _ = client.New(context.Background(), client1)
		mycli2, _ = client.New(context.Background(), client2)
		mycli3, _ = client.New(context.Background(), client3)
}

func setupCluster() protocol.NodeStore {
	// set the inmemnodestore to refer the cluster
	store := client.NewInmemNodeStore()
	store.Set(context.Background(), []client.NodeInfo{{Address: client1}, {Address: client2}, {Address: client3}})
	//return store

	// prepare node 2 and 3 to be added to the leader
	// the leader by default has ID = 1 or BootstrapID (some hardcoded value)
	nodeinf2 := client.NodeInfo{ID: 2, Address: client2, Role: client.Voter}
	nodeinf3 := client.NodeInfo{ID: 3, Address: client3, Role: client.Voter}

	// find the leader client (should be the same as mycli1)
	leadercli, _ := client.FindLeader(context.Background(), store, []client.Option{client.WithDialFunc(client.DefaultDialFunc)}...)

	// add node2
	err := leadercli.Add(context.Background(), nodeinf2)
	if err != nil {
		fmt.Errorf("Cannot add node 2 %s\n", err)
	}
	// and node3
	err = leadercli.Add(context.Background(), nodeinf3)
	if err != nil {
		fmt.Errorf("Cannot add node 3 %s\n", err)
	}

	return store
}

func printCluster() {
	var leader_ni *protocol.NodeInfo
	fmt.Println("Printing cluster..")

	if mycli1 != nil {
		fmt.Println("From node 1")
		leader_ni, _ = mycli1.Leader(context.Background())
		fmt.Println(leader_ni.ID, " at ", leader_ni.Address)
		servers, _ := mycli1.Cluster(context.Background())
		for _, ni := range servers {
			fmt.Printf("%s--%s,", ni.Address, ni.Role)
		}
		fmt.Println("\n-----------------")
	}
	if mycli2 != nil {
		fmt.Println("From node 2")
		leader_ni, _ = mycli2.Leader(context.Background())
		fmt.Println(leader_ni.ID, " at ", leader_ni.Address)
		servers, _ := mycli2.Cluster(context.Background())
		for _, ni := range servers {
			fmt.Printf("%s--%s,", ni.Address, ni.Role)
		}
		fmt.Println("\n-----------------")
	}
	if mycli3 != nil {
		fmt.Println("From node 3")
		leader_ni, _ = mycli3.Leader(context.Background())
		fmt.Println(leader_ni.ID, " at ", leader_ni.Address)
		servers, _ := mycli3.Cluster(context.Background())
		for _, ni := range servers {
			fmt.Printf("%s--%s,", ni.Address, ni.Role)
		}
		fmt.Println("\n-----------------")
	}

}

func loadSQLFile(db *sql.DB, sqlFile string) error {
	file, err := ioutil.ReadFile(sqlFile)
	if err != nil {
		return err
	}
	var count int
	currentTime := time.Now()
	tx, err := db.Begin()
	for _, q := range strings.Split(string(file), ";") {

		if err != nil {
			return err
		}
		q := strings.TrimSpace(q)
		if q == "" {
			continue
		}

		count = count + 1
		if count%50000 == 0 {
			diff := currentTime.Sub(time.Now())
			fmt.Printf("%d in %f secs\n", count, diff.Seconds())
			tx.Commit()

			// time.Sleep(2 * time.Second)

			currentTime = time.Now()
			tx, _ = db.Begin()
		}

		// db.SetMaxOpenConns(1)
		// db.SetMaxIdleConns(1)

		if _, err := tx.Exec(q); err != nil {
			fmt.Println("Error in ", q, err)
			return err
		}

		if strings.HasPrefix(q, "CREATE") {
			fmt.Println(q)
			tx.Commit()
			tx, _ = db.Begin()
		}

	}
	fmt.Println("done!", count)
	return nil
}

func main() {
	var api string
	var verbose bool

	cmd := &cobra.Command{
		Use:   "dqlite-client",
		Short: "Dqlite client test with existing node somewhere else",
		RunE: func(cmd *cobra.Command, args []string) error {
			// connect clients so we can use later
			connectClients()

			// setup the cluster
			store := setupCluster()
			// open sql driver based on the store we have
			driver, err := driver.New(store)
			if err != nil {
				log.Fatal(err)
			}
			// register driver
			sql.Register("dqlite", driver)
			// open db to work with
			db, err := sql.Open("dqlite", "db_name")
			if err != nil {
				log.Fatal(err)
			}
			defer db.Close()

			// print cluster to see current info
			printCluster()

			// db.Exec("DROP TABLE IF EXISTS biggy")
			// test := "CREATE TABLE IF NOT EXISTS biggy AS select * from employees e, salaries s, dept_emp de, departments d WHERE e.emp_id = de.emp_id AND de.dept_id = d.dept_id AND s.emp_id = e.emp_id;"
			// _, err = db.Exec(string(test))
			// if err != nil {
			// 	fmt.Println("can't create biggy")
			// 	return err
			// }

			// // table 'check'
			rows, err := db.Query("select * from sqlite_master")
			if err != nil {
				return err
			}
			for rows.Next() {
				fmt.Println(rows)
			}

			// workload
			start := time.Now()
			rows, err := db.Query("select max(emp_id) from employees")
			if err != nil {
				return err
			}
			t := time.Now()	
			fmt.Println(t.Sub(start).Milliseconds())
			for rows.Next() {
				fmt.Println(rows)
			}

			// rows, err = db.Query(`select count(*) FROM (select total(birth_date), emp_id from employees WHERE hire_date > '1985-11-11' OR last_name LIKE '%i' AND random()+emp_id % 999999 > 0 GROUP BY birth_date) zz, employees e WHERE e.emp_id = zz.emp_id AND e.hire_date > '1985-09-11' AND e.first_name LIKE 'P%';`)
			// t := time.Now()
			// if err != nil {
			// 	return err
			// }
			// fmt.Println(t.Sub(start).Milliseconds())

			// for rows.Next() {
			// 	// 	fmt.Println(rows)
			// }

			// loadSQLFile(db, "emp.sql")

			// create key-value table if not exist
			// if _, err := db.Exec(string(schema)); err != nil {
			// 	return err
			// }


			http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				// key := strings.TrimLeft(r.URL.Path, "/")
				result := ""
				switch r.Method {
				case "GET":
					tx, _ := db.Begin()
					var q string
					for i := 0; i < 10; i++ {
						i := 799999 + rand.Intn(100000)
						cs := 'a' + rune(rand.Intn(26))
						cb := 'A' + rune(rand.Intn(26))
						q = fmt.Sprintf(`select count(*) FROM (select total(birth_date), emp_id from employees WHERE hire_date > '1985-11-11' OR last_name LIKE '%%%c' AND random()+emp_id %% %d > 0 GROUP BY birth_date) zz, employees e WHERE e.emp_id = zz.emp_id AND e.hire_date > '1985-09-11' AND e.first_name LIKE '%c%%';`, cs, i, cb)
						// fmt.Println(q)
						_, err = tx.Query(q)
					}
					err = tx.Commit()

					if err != nil {
						w.WriteHeader(http.StatusInternalServerError)
						result = fmt.Sprintf("Error: %s", err.Error())
					} else {
						result = "ok"
					}

					break
				// case "PUT":
				// 	result = "done"
				// 	value, _ := ioutil.ReadAll(r.Body)
				// 	if _, err := db.Exec(update, key, value); err != nil {
				// 		result = fmt.Sprintf("Error: %s", err.Error())
				// 	}
				default:
					result = fmt.Sprintf("Error: unsupported method %q", r.Method)

				}
				fmt.Fprintf(w, "%s\n", result)
			})

			listener, err := net.Listen("tcp", api)
			if err != nil {
				return err
			}

			go http.Serve(listener, nil)

			ch := make(chan os.Signal)
			signal.Notify(ch, unix.SIGPWR)
			signal.Notify(ch, unix.SIGINT)
			signal.Notify(ch, unix.SIGQUIT)
			signal.Notify(ch, unix.SIGTERM)

			<-ch

			listener.Close()
			db.Close()

			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&api, "api", "a", "", "address used to expose the demo API")
	flags.BoolVarP(&verbose, "verbose", "v", false, "verbose logging")

	cmd.MarkFlagRequired("api")

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
