package cli

import (
	"github.com/CMonnin/dwkt/internal/session"
	"github.com/CMonnin/dwkt/internal/tui"
)

func runTUI(s *session.Session) error { return tui.Run(s) }
