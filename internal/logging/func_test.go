package logging_test

import (
	"testing"

	"github.com/ardhipoetra/go-dqlite/internal/logging"
)

func Test_TestFunc(t *testing.T) {
	f := logging.Test(t)
	f(logging.Info, "hello")
}
