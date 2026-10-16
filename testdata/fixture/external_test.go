package fixture_test

import (
	"os"
	"testing"
)

func TestExternal(t *testing.T) { _ = os.Remove("not-executed") }
