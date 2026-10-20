// Package investigation is analyzed, never executed by the CLI fixtures.
package investigation

import (
	"io"
	"log"
	"os"
	"sync"
)

func ConcreteClose(f *os.File) error         { return f.Close() }
func InterfaceClose(c io.Closer) error       { return c.Close() }
func StreamClose(c io.ReadWriteCloser) error { return c.Close() }
func BoundClose(c io.Closer) error           { close := c.Close; return close() }

type Embedded interface{ io.Closer }

func EmbeddedClose(c Embedded) error { return c.Close() }

type Lookalike interface{ Close() error }

func UnknownClose(c Lookalike) error { return c.Close() }

// Persistence's contract is application-owned, not a recognized storage kind.
type Persistence interface{ Save([]byte) error }

func Persist(p Persistence) error {
	log.Print("saving")
	return p.Save(nil)
}
func CallerA(p Persistence) error     { return Persist(p) }
func CallerB(p Persistence) error     { return Persist(p) }
func Independent(p Persistence) error { return p.Save(nil) }

func Rename() error               { return os.Rename("never-old", "never-new") }
func RootRename(r *os.Root) error { return r.Rename("never-old", "never-new") }

type Renamer interface{ Rename(string, string) error }

func UnknownRename(r Renamer) error { return r.Rename("never-old", "never-new") }

// Location plus reverse callers let investigators inspect this local lock scope.
func LockedClose(mu *sync.Mutex, c io.ReadWriteCloser) error {
	mu.Lock()
	defer mu.Unlock()
	return c.Close()
}
func CloseCaller(mu *sync.Mutex, c io.ReadWriteCloser) error { return LockedClose(mu, c) }
