package dep

import "os"

func Load() { _, _ = os.ReadFile("not-executed") }
