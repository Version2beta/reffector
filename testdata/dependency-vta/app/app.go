package dependencyfixture

import (
	"os"

	"example.com/dependency"
)

type Disk struct{}

func (Disk) Handle() { _ = os.Remove("disk") }

type Memory struct{}

func (Memory) Handle() {}

type Similar struct{}

func (Similar) Handle(_ int) {}

func Root() {
	dependency.Dispatch(Disk{})
	dependency.Callback(func() { _ = os.WriteFile("callback", nil, 0600) })
}

func OpenBoundary(handler dependency.Handler) {
	dependency.Dispatch(handler)
}
