package cli

import (
	stdcontext "context"
	"os"
	"os/signal"
)

// commandContext returns the context shared by every Jira call of one
// command. It is cancelled on Ctrl-C so in-flight requests stop cleanly.
// The caller must call the returned stop function when the command ends.
func commandContext() (stdcontext.Context, stdcontext.CancelFunc) {
	return signal.NotifyContext(stdcontext.Background(), os.Interrupt)
}
