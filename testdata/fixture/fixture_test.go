package fixture

import (
	"os"
	"testing"
)

func TestOnly(t *testing.T) { _ = os.Setenv("NOT_EXECUTED", "1") }
