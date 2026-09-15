package cli

import (
	"github.com/CMonnin/dkt/internal/session"
	"github.com/CMonnin/dkt/internal/tui"
)

func runTUI(s *session.Session) error { return tui.Run(s) }
