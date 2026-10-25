// Auxesis Capital: the public site and investor portal, as a PocketBase app with custom routes.
// Sign-in comes from the students.iima.ac.in platform (iimakit/pb); this app decides who is an
// admin, investor or guest.
//
//	go run . serve                    # dev, http://127.0.0.1:8090 (dashboard at /_/)
//	go build -o auxesis-server && ./auxesis-server serve --http 127.0.0.1:8091
package main

import (
	"log"

	"github.com/pocketbase/pocketbase/core"

	iimakit "github.com/arnav127/iima-platform/kit"
	"github.com/arnav127/iima-platform/kit/pb"

	"github.com/arnav127/auxesis-capital/backend/fund"
	_ "github.com/arnav127/auxesis-capital/backend/migrations"
)

func main() {
	var s *fund.Service
	app := pb.New(pb.Config{
		AppID:   "auxesiscapital",
		AppName: "Auxesis Capital",
		Access: func(e *core.RequestEvent, c *iimakit.Claims) (*core.Record, error) {
			return s.Access(c.Email, c.Name)
		},
		OnFirstSeen: func(e *core.RequestEvent, u *core.Record, c *iimakit.Claims) {
			s.SignedIn(e, u)
		},
	})
	s = fund.Register(app)
	app.RootCmd.AddCommand(fund.Commands(app, s)...)

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
