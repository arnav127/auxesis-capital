package fund

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/filesystem"
)

// Register wires the Auxesis routes, sign-in rules and the daily sync into PocketBase.
func Register(app core.App) *Service {
	s := New(app)
	s.ExcelURL = strings.TrimSpace(os.Getenv("EXCEL_URL"))
	s.TradesSheet = strings.TrimSpace(os.Getenv("TRADES_SHEET"))
	if !strings.EqualFold(os.Getenv("PRICE_HISTORY"), "off") {
		s.Yahoo = &Yahoo{BaseURL: os.Getenv("YAHOO_BASE_URL")}
	}

	// Sign-in is Google OAuth2 only, and only for people on the investor list or ADMIN_EMAILS.
	app.OnRecordAuthWithOAuth2Request("users").BindFunc(func(e *core.RecordAuthWithOAuth2RequestEvent) error {
		if e.OAuth2User == nil || e.OAuth2User.Email == "" {
			return apis.NewForbiddenError(notRegistered, nil)
		}
		u, err := s.SignIn(e.OAuth2User.Email, e.OAuth2User.Name)
		if err != nil {
			return err
		}
		e.Record, e.IsNewRecord = u, false
		return e.Next()
	})

	// Anything that changes the numbers drops the cached snapshot.
	for _, c := range []string{"capital_flows", "investor_txns", "prices", "instruments", "fund_settings", "trades", "other_pnl"} {
		inval := func(e *core.RecordEvent) error { s.Invalidate(); return e.Next() }
		app.OnRecordAfterCreateSuccess(c).BindFunc(inval)
		app.OnRecordAfterUpdateSuccess(c).BindFunc(inval)
		app.OnRecordAfterDeleteSuccess(c).BindFunc(inval)
	}

	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		if err := applyEnvConfig(app); err != nil {
			return err
		}
		if err := s.ensureAdmins(); err != nil {
			return err
		}
		s.schedule()

		r := se.Router
		g := r.Group("/api/aux")
		auth := apis.RequireAuth("users")
		reply := func(e *core.RequestEvent, v any, err error) error {
			if err != nil {
				return err
			}
			return e.JSON(http.StatusOK, v)
		}

		g.GET("/public", func(e *core.RequestEvent) error {
			e.Response.Header().Set("Cache-Control", "public, max-age=60")
			return e.JSON(200, s.Public())
		})
		g.GET("/me", func(e *core.RequestEvent) error {
			return e.JSON(200, s.me(e.Auth))
		}).Bind(auth)
		g.GET("/portfolio", func(e *core.RequestEvent) error {
			v, err := s.Portfolio(e.Auth, isAdmin(e.Auth))
			return reply(e, v, err)
		}).Bind(auth)
		g.GET("/reports", func(e *core.RequestEvent) error {
			v, err := s.Reports(signedIn(e))
			return reply(e, v, err)
		})
		g.GET("/reports/{slug}", func(e *core.RequestEvent) error {
			rep, _, err := s.Report(e.Request.PathValue("slug"), signedIn(e))
			if err != nil {
				return err
			}
			if rep == nil {
				return apis.NewNotFoundError("That publication doesn't exist.", nil)
			}
			return e.JSON(200, rep)
		})
		g.GET("/reports/{slug}/pdf", func(e *core.RequestEvent) error {
			rep, rec, err := s.Report(e.Request.PathValue("slug"), signedIn(e))
			if err != nil {
				return err
			}
			if rep == nil || !rep.HasPDF {
				return apis.NewNotFoundError("There is no PDF for this publication.", nil)
			}
			if rep.Locked {
				return apis.NewForbiddenError("Sign in as an investor to download this PDF.", nil)
			}
			fsys, err := app.NewFilesystem()
			if err != nil {
				return err
			}
			defer fsys.Close()
			name := rec.GetString("pdf")
			e.Response.Header().Set("Cache-Control", "private, no-store")
			return fsys.Serve(e.Response, e.Request, rec.BaseFilesPath()+"/"+name, rep.Slug+".pdf")
		})
		g.GET("/reports/{slug}/img/{name}", func(e *core.RequestEvent) error {
			rec, _ := app.FindFirstRecordByData("reports", "slug", e.Request.PathValue("slug"))
			name := e.Request.PathValue("name")
			if rec == nil || (!rec.GetBool("published") && !isAdmin(e.Auth)) || !slices.Contains(rec.GetStringSlice("images"), name) {
				return apis.NewNotFoundError("No such image.", nil)
			}
			if rec.GetString("access") == "investors" && !signedIn(e) {
				return apis.NewForbiddenError("Sign in as an investor to see this image.", nil)
			}
			fsys, err := app.NewFilesystem()
			if err != nil {
				return err
			}
			defer fsys.Close()
			e.Response.Header().Set("Cache-Control", "private, max-age=86400")
			return fsys.Serve(e.Response, e.Request, rec.BaseFilesPath()+"/"+name, name)
		})
		g.GET("/team", func(e *core.RequestEvent) error {
			v, err := s.Team()
			return reply(e, v, err)
		})

		// ---- admins ----
		admin := func(e *core.RequestEvent) error {
			if !isAdmin(e.Auth) {
				return apis.NewForbiddenError("Only fund admins can do this.", nil)
			}
			return e.Next()
		}
		g.GET("/admin/status", func(e *core.RequestEvent) error {
			v, err := s.Admin()
			return reply(e, v, err)
		}).Bind(auth).BindFunc(admin)
		g.POST("/admin/sync", func(e *core.RequestEvent) error {
			ctx, cancel := context.WithTimeout(e.Request.Context(), 90*time.Second)
			defer cancel()
			sum, err := s.SyncFromLink(ctx, e.Auth.GetString("email"))
			if err != nil {
				return apis.NewBadRequestError(err.Error(), nil)
			}
			go s.backfillAsync(false)
			return e.JSON(200, sum)
		}).Bind(auth).BindFunc(admin)
		g.POST("/admin/upload", func(e *core.RequestEvent) error {
			data, err := uploaded(e, "file")
			if err != nil {
				return err
			}
			sum, err := s.Import(data, "upload", e.Auth.GetString("email"))
			if err != nil {
				return apis.NewBadRequestError(err.Error(), nil)
			}
			go s.backfillAsync(false)
			return e.JSON(200, sum)
		}).Bind(auth, apis.BodyLimit(64<<20)).BindFunc(admin)
		g.POST("/admin/investors", func(e *core.RequestEvent) error {
			var in InvestorInput
			if err := e.BindBody(&in); err != nil {
				return apis.NewBadRequestError("Invalid request.", nil)
			}
			r, err := s.SaveInvestor(in)
			if err != nil {
				return apis.NewBadRequestError(err.Error(), nil)
			}
			return e.JSON(200, map[string]string{"id": r.Id, "message": "Saved " + r.GetString("name") + "."})
		}).Bind(auth).BindFunc(admin)
		g.POST("/admin/investors/bulk", func(e *core.RequestEvent) error {
			var body struct {
				Rows []InvestorInput `json:"rows"`
			}
			if err := e.BindBody(&body); err != nil {
				return apis.NewBadRequestError("Invalid request.", nil)
			}
			n, err := s.SaveInvestors(body.Rows)
			if err != nil {
				return apis.NewBadRequestError(err.Error(), nil)
			}
			return e.JSON(200, map[string]string{"message": fmt.Sprintf("Saved %d investors.", n)})
		}).Bind(auth).BindFunc(admin)
		g.DELETE("/admin/investors/{id}", func(e *core.RequestEvent) error {
			if err := s.DeleteInvestor(e.Request.PathValue("id")); err != nil {
				return apis.NewBadRequestError(err.Error(), nil)
			}
			return e.JSON(200, map[string]string{"message": "Removed."})
		}).Bind(auth).BindFunc(admin)
		g.POST("/admin/investors/import", func(e *core.RequestEvent) error {
			data, err := uploaded(e, "file")
			if err != nil {
				return err
			}
			v, err := ParseInvestors(data)
			if err != nil {
				return apis.NewBadRequestError(err.Error(), nil)
			}
			return e.JSON(200, v)
		}).Bind(auth, apis.BodyLimit(64<<20)).BindFunc(admin)
		g.POST("/admin/investors/import/confirm", func(e *core.RequestEvent) error {
			var body struct {
				Rows []InvestorInput `json:"rows"`
			}
			if err := e.BindBody(&body); err != nil {
				return apis.NewBadRequestError("Invalid request.", nil)
			}
			sum, err := s.ImportInvestors(body.Rows)
			if err != nil {
				return apis.NewBadRequestError(err.Error(), nil)
			}
			return e.JSON(200, sum)
		}).Bind(auth).BindFunc(admin)
		g.POST("/admin/flows", func(e *core.RequestEvent) error {
			var f FlowRow
			if err := e.BindBody(&f); err != nil {
				return apis.NewBadRequestError("Invalid request.", nil)
			}
			if err := s.AddFlow(f); err != nil {
				return apis.NewBadRequestError(err.Error(), nil)
			}
			return e.JSON(200, map[string]string{"message": "Saved."})
		}).Bind(auth).BindFunc(admin)
		g.DELETE("/admin/flows/{id}", func(e *core.RequestEvent) error {
			if err := s.DeleteFlow(e.Request.PathValue("id")); err != nil {
				return apis.NewBadRequestError(err.Error(), nil)
			}
			return e.JSON(200, map[string]string{"message": "Removed."})
		}).Bind(auth).BindFunc(admin)

		// ---- the report editor ----
		g.GET("/admin/reports", func(e *core.RequestEvent) error {
			v, err := s.AllReports()
			return reply(e, v, err)
		}).Bind(auth).BindFunc(admin)
		g.GET("/admin/reports/{id}", func(e *core.RequestEvent) error {
			v, err := s.ReportDraftByID(e.Request.PathValue("id"))
			if err != nil {
				return apis.NewNotFoundError(err.Error(), nil)
			}
			return e.JSON(200, v)
		}).Bind(auth).BindFunc(admin)
		g.POST("/admin/reports", func(e *core.RequestEvent) error {
			var d ReportDraft
			if err := json.Unmarshal([]byte(e.Request.FormValue("data")), &d); err != nil {
				return apis.NewBadRequestError("Invalid request.", nil)
			}
			var pdf *filesystem.File
			if files, _ := e.FindUploadedFiles("pdf"); len(files) > 0 {
				pdf = files[0]
			}
			v, err := s.SaveReport(d, pdf, e.Request.FormValue("removePdf") == "1")
			if err != nil {
				return apis.NewBadRequestError(err.Error(), nil)
			}
			return e.JSON(200, v)
		}).Bind(auth, apis.BodyLimit(60<<20)).BindFunc(admin)
		g.POST("/admin/reports/{id}/images", func(e *core.RequestEvent) error {
			files, err := e.FindUploadedFiles("images")
			if err != nil || len(files) == 0 {
				return apis.NewBadRequestError("Choose an image to upload.", nil)
			}
			names, err := s.AddReportImages(e.Request.PathValue("id"), files)
			if err != nil {
				return apis.NewBadRequestError(err.Error(), nil)
			}
			return e.JSON(200, map[string]any{"names": names})
		}).Bind(auth, apis.BodyLimit(60<<20)).BindFunc(admin)
		g.DELETE("/admin/reports/{id}", func(e *core.RequestEvent) error {
			if err := s.DeleteReport(e.Request.PathValue("id")); err != nil {
				return apis.NewBadRequestError(err.Error(), nil)
			}
			return e.JSON(200, map[string]string{"message": "Deleted."})
		}).Bind(auth).BindFunc(admin)

		g.POST("/admin/prices", func(e *core.RequestEvent) error {
			if s.Yahoo == nil {
				return apis.NewBadRequestError("Price history downloads are off (PRICE_HISTORY=off in .env).", nil)
			}
			go s.backfillAsync(e.Request.URL.Query().Get("full") == "1")
			return e.JSON(202, map[string]string{"message": "Downloading price history. This takes a minute; the log below updates when it's done."})
		}).Bind(auth).BindFunc(admin)

		return se.Next()
	})
	return s
}

// uploaded reads one uploaded file from a multipart form.
func uploaded(e *core.RequestEvent, field string) ([]byte, error) {
	files, err := e.FindUploadedFiles(field)
	if err != nil || len(files) == 0 {
		return nil, apis.NewBadRequestError("Choose the file to upload.", nil)
	}
	rc, err := files[0].Reader.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(io.LimitReader(rc, 64<<20))
}

const notRegistered = "This Google account isn't registered with Auxesis Capital. Sign in with the email you gave the fund, or write to the Investments Cell at Beta."

func signedIn(e *core.RequestEvent) bool { return e.Auth != nil && e.Auth.Collection().Name == "users" }

func isAdmin(u *core.Record) bool { return u != nil && u.GetString("role") == "admin" }

type MeView struct {
	User struct {
		Name  string `json:"name"`
		Email string `json:"email"`
		Role  string `json:"role"`
	} `json:"user"`
	Investor *struct {
		Name  string `json:"name"`
		Folio string `json:"folio"`
	} `json:"investor"`
}

func (s *Service) me(u *core.Record) MeView {
	var v MeView
	v.User.Name, v.User.Email, v.User.Role = u.GetString("name"), u.GetString("email"), u.GetString("role")
	if inv := s.InvestorFor(v.User.Email); inv != nil {
		v.Investor = &struct {
			Name  string `json:"name"`
			Folio string `json:"folio"`
		}{inv.GetString("name"), inv.GetString("folio")}
		if v.User.Name == "" {
			v.User.Name = inv.GetString("name")
		}
	}
	return v
}

func adminEmails() map[string]bool {
	out := map[string]bool{}
	for _, e := range strings.Split(os.Getenv("ADMIN_EMAILS"), ",") {
		if e = strings.ToLower(strings.TrimSpace(e)); e != "" {
			out[e] = true
		}
	}
	return out
}

// SignIn returns the user for an email if they may sign in, creating the account on first use.
func (s *Service) SignIn(email, name string) (*core.Record, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	role := ""
	if adminEmails()[email] {
		role = "admin"
	} else if inv := s.InvestorFor(email); inv != nil {
		role = "investor"
		if name == "" {
			name = inv.GetString("name")
		}
	}
	if role == "" {
		return nil, apis.NewForbiddenError(notRegistered, nil)
	}
	u, err := s.app.FindAuthRecordByEmail("users", email)
	if err != nil {
		c, err := s.app.FindCollectionByNameOrId("users")
		if err != nil {
			return nil, err
		}
		u = core.NewRecord(c)
		u.SetEmail(email)
		u.SetVerified(true)
		u.SetRandomPassword()
	}
	changed := u.IsNew()
	if u.GetString("role") != role {
		u.Set("role", role)
		changed = true
	}
	if name != "" && u.GetString("name") == "" {
		u.Set("name", name)
		changed = true
	}
	if changed {
		if err := s.app.Save(u); err != nil {
			return nil, err
		}
	}
	return u, nil
}

// ensureAdmins demotes people removed from ADMIN_EMAILS.
func (s *Service) ensureAdmins() error {
	admins := adminEmails()
	rows, err := s.app.FindRecordsByFilter("users", "role = 'admin'", "", 0, 0)
	if err != nil {
		return err
	}
	for _, u := range rows {
		if !admins[strings.ToLower(u.Email())] {
			u.Set("role", "investor")
			if err := s.app.Save(u); err != nil {
				return err
			}
		}
	}
	return nil
}

// schedule syncs the tracker after market close on weekdays and downloads the day's closes.
//
//	SYNC_CRON   when to pull the tracker (IST), default "10 16,19,22 * * 1-5"
func (s *Service) schedule() {
	c := s.app.Cron()
	c.SetTimezone(IST)
	expr := strings.TrimSpace(os.Getenv("SYNC_CRON"))
	if expr == "" {
		expr = "10 16,19,22 * * 1-5"
	}
	if expr == "off" {
		return
	}
	if err := c.Add("auxesis-sync", expr, func() { s.scheduledSync() }); err != nil {
		s.app.Logger().Error("bad SYNC_CRON", slog.String("expr", expr), slog.String("error", err.Error()))
	}
	// First start (or after a restart): catch up in the background.
	go func() {
		time.Sleep(3 * time.Second)
		s.scheduledSync()
	}()
}

func (s *Service) scheduledSync() {
	if s.ExcelURL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		_, err := s.SyncFromLink(ctx, "schedule")
		cancel()
		if err != nil && !errors.Is(err, ErrNeedsSignIn) {
			s.app.Logger().Warn("scheduled sync", slog.String("error", err.Error()))
		}
	}
	s.backfillAsync(false)
}

func (s *Service) backfillAsync(full bool) {
	if s.Yahoo == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	s.BackfillPrices(ctx, full)
	if err := s.Recompute(); err != nil {
		s.app.Logger().Error("recompute", slog.String("error", err.Error()))
	}
}
