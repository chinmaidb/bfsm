package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"schemeapp/backend/internal/application"
	"schemeapp/backend/internal/config"
	"schemeapp/backend/internal/security"
)

type API struct {
	service *application.Service
	tokens  *security.TokenIssuer
	logger  *slog.Logger
	config  config.Config
}

func serviceError(err error) error {
	var input application.InputError
	if errors.As(err, &input) {
		return echo.NewHTTPError(http.StatusBadRequest, input.Error())
	}
	if errors.Is(err, sql.ErrNoRows) {
		return echo.NewHTTPError(http.StatusNotFound, "record not found")
	}
	return err
}

func New(cfg config.Config, logger *slog.Logger, service *application.Service, tokens *security.TokenIssuer) *echo.Echo {
	a := &API{service: service, tokens: tokens, logger: logger, config: cfg}
	e := echo.New()
	e.Use(middleware.RequestLogger())
	e.Use(middleware.Recover())
	e.Use(middleware.RequestIDWithConfig(middleware.RequestIDConfig{RequestIDHandler: func(c *echo.Context, requestID string) { c.Set("requestID", requestID) }}))
	e.Use(middleware.BodyLimit(1 << 20))
	origins := strings.Split(cfg.CORSAllowedOrigins, ",")
	for i := range origins {
		origins[i] = strings.TrimSpace(origins[i])
	}
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{AllowOrigins: origins, AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions}, AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization, echo.HeaderXRequestID}}))
	e.GET("/health/live", func(c *echo.Context) error { return c.JSON(http.StatusOK, map[string]string{"status": "ok"}) })
	e.GET("/health/ready", func(c *echo.Context) error {
		if err := service.DB.PingContext(c.Request().Context()); err != nil {
			return c.JSON(http.StatusServiceUnavailable, map[string]string{"status": "not_ready"})
		}
		return c.JSON(http.StatusOK, map[string]string{"status": "ready"})
	})
	e.GET("/api/v1", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"name": "benefit-scheme-api", "environment": cfg.Environment})
	})
	e.GET("/api/v1/catalog/schemes", a.publicSchemes)
	loginLimiter := middleware.NewRateLimiterMemoryStoreWithConfig(middleware.RateLimiterMemoryStoreConfig{Rate: 0.2, Burst: 5})
	e.POST("/api/v1/auth/login", a.login, middleware.RateLimiter(loginLimiter))
	e.GET("/api/v1/auth/me", a.currentAccess, a.authenticate("admin", "staff", "promoter", "member"))

	admin := e.Group("/api/v1", a.authenticate("admin", "staff"))
	for _, kind := range []string{"promoters", "members", "schemes"} {
		resource := kind
		admin.GET("/"+resource, a.list(resource), a.requirePermission(resource+".read"))
		admin.POST("/"+resource, a.create(resource), a.requirePermission(resource+".create"))
		admin.PUT("/"+resource+"/:id", a.update(resource), a.requirePermission(resource+".update"))
		admin.DELETE("/"+resource+"/:id", a.deactivate(resource), a.requirePermission(resource+".delete"))
	}
	admin.POST("/enrollments", a.createEnrollment, a.requirePermission("enrollments.create"))
	admin.GET("/enrollments", a.enrollments, a.requirePermission("enrollments.read"))
	admin.PATCH("/enrollments/:id/status", a.updateEnrollmentStatus, a.requirePermission("enrollments.update"))
	admin.POST("/draw-results", a.recordDraw, a.requirePermission("draws.create"))
	admin.POST("/promoter-incentives", a.createIncentive, a.requirePermission("incentives.create"))
	admin.PATCH("/promoter-incentives/:id/status", a.updateIncentiveStatus, a.requirePermission("incentives.update"))
	admin.GET("/reports/:kind", a.report, a.requireReportPermission())
	admin.GET("/audit", a.audit, a.requirePermission("audit.read"))
	admin.GET("/parties", a.listParties, a.requirePermission("parties.read"))
	admin.POST("/parties", a.createParty, a.requirePermission("parties.create"))
	admin.PUT("/parties/:id", a.updateParty, a.requirePermission("parties.update"))
	admin.DELETE("/parties/:id", a.deactivateParty, a.requirePermission("parties.delete"))
	admin.GET("/cash-receipts", a.listCustomerReceipts, a.requirePermission("cash_receipts.read"))
	admin.POST("/cash-receipts", a.recordCustomerReceipt, a.requirePermission("cash_receipts.create"))
	admin.GET("/supplier-payments", a.listSupplierPayments, a.requirePermission("supplier_payments.read"))
	admin.POST("/supplier-payments", a.recordSupplierPayment, a.requirePermission("supplier_payments.create"))
	admin.GET("/financial-entries/:id/receipt", a.financialReceipt, a.requireAnyPermission("cash_receipts.read", "supplier_payments.read"))
	admin.GET("/accounts", a.listAccounts, a.requirePermission("accounts.read"))
	admin.POST("/accounts", a.createAccount, a.requirePermission("accounts.create"))
	admin.PUT("/accounts/:id", a.updateAccount, a.requirePermission("accounts.update"))
	admin.DELETE("/accounts/:id", a.deleteAccount, a.requirePermission("accounts.delete"))
	admin.GET("/roles", a.listRoles, a.requirePermission("roles.read"))
	admin.POST("/roles", a.createRole, a.requirePermission("roles.create"))
	admin.PUT("/roles/:id", a.updateRole, a.requirePermission("roles.update"))
	admin.DELETE("/roles/:id", a.deleteRole, a.requirePermission("roles.delete"))
	admin.GET("/permissions", a.permissions, a.requirePermission("roles.read"))

	staff := e.Group("/api/v1", a.authenticate("admin", "promoter", "staff"))
	staff.POST("/payments", a.recordPayment, a.requirePermissionOrRole("payments.create", "promoter"))
	staff.GET("/payments/:id/receipt", a.receipt, a.requirePermissionOrRole("payments.read", "promoter"))
	staff.GET("/payments/mine", a.paymentsMine, a.requirePermissionOrRole("payments.read", "promoter"))
	staff.GET("/enrollments/mine", a.enrollmentsMine, a.requirePermissionOrRole("enrollments.read", "promoter"))
	staff.GET("/members/mine", a.myMembers, a.requirePermissionOrRole("members.read", "promoter"))
	staff.POST("/members/mine", a.createMyCustomer, a.requirePermissionOrRole("members.create", "promoter"))
	staff.POST("/promoter/enrollments", a.createEnrollment, a.requirePermissionOrRole("enrollments.create", "promoter"))
	staff.GET("/promoter/incentives/mine", a.myIncentives, a.requirePermissionOrRole("incentives.read", "promoter"))
	member := e.Group("/api/v1", a.authenticate("member"))
	member.GET("/member/me", a.myMember, a.requirePermission("dashboard.read"))
	member.GET("/member/payments", a.myPayments, a.requirePermission("payments.read"))
	member.GET("/member/payments/:id/receipt", a.receipt, a.requirePermission("payments.read"))
	member.GET("/member/enrollments", a.enrollmentsMine, a.requirePermission("enrollments.read"))
	member.PUT("/member/enrollments/:id/frequency", a.setMyPaymentFrequency, a.requirePermission("enrollments.frequency.update"))

	e.HTTPErrorHandler = func(c *echo.Context, err error) {
		var he *echo.HTTPError
		if errors.As(err, &he) {
			echo.DefaultHTTPErrorHandler(false)(c, err)
			return
		}
		logger.Error("request failed", "error", err, "path", c.Request().URL.Path)
		echo.DefaultHTTPErrorHandler(false)(c, echo.NewHTTPError(http.StatusInternalServerError, "internal server error"))
	}
	return e
}

func (a *API) actor(c *echo.Context) application.Actor {
	claims, _ := c.Get("claims").(*security.AccessClaims)
	requestID, _ := c.Get("requestID").(string)
	actor := application.Actor{RequestID: requestID}
	if claims != nil {
		actor.UserID = claims.Subject
		actor.Role = claims.Role
	}
	return actor
}

func (a *API) authenticate(roles ...string) echo.MiddlewareFunc {
	allowed := map[string]bool{}
	for _, r := range roles {
		allowed[r] = true
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			claims, err := a.requestClaims(c)
			if err != nil { return err }
			if !allowed[claims.Role] {
				return echo.NewHTTPError(http.StatusForbidden, "insufficient permissions")
			}
			return next(c)
		}
	}
}

func (a *API) requestClaims(c *echo.Context) (*security.AccessClaims, error) {
	if claims, ok := c.Get("claims").(*security.AccessClaims); ok && claims != nil { return claims, nil }
	h := strings.TrimSpace(c.Request().Header.Get(echo.HeaderAuthorization))
	if !strings.HasPrefix(h, "Bearer ") { return nil, echo.NewHTTPError(http.StatusUnauthorized, "authentication required") }
	claims, err := a.tokens.Verify(strings.TrimSpace(strings.TrimPrefix(h, "Bearer ")))
	if err != nil { return nil, echo.NewHTTPError(http.StatusUnauthorized, "invalid or expired access token") }
	c.Set("claims", claims)
	return claims, nil
}

func (a *API) requirePermission(permission string) echo.MiddlewareFunc {
	return a.requirePermissionOrRole(permission)
}

func (a *API) requireAnyPermission(permissions ...string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			claims, err := a.requestClaims(c)
			if err != nil { return err }
			for _, permission := range permissions {
				allowed, err := a.service.Can(c.Request().Context(), claims.Subject, permission)
				if err == nil && allowed { return next(c) }
			}
			return echo.NewHTTPError(http.StatusForbidden, "insufficient permissions")
		}
	}
}

func (a *API) requireReportPermission() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			if c.Param("kind") == "cashbook" {
				claims, err := a.requestClaims(c)
				if err != nil { return err }
				for _, permission := range []string{"cash_receipts.read", "supplier_payments.read"} {
					allowed, checkErr := a.service.Can(c.Request().Context(), claims.Subject, permission)
					if checkErr != nil || !allowed { return echo.NewHTTPError(http.StatusForbidden, "insufficient permissions") }
				}
				return next(c)
			}
			permissions := []string{"reports.read"}
			switch c.Param("kind") {
			case "payments", "daybook": permissions = append(permissions, "payments.read")
			case "dues": permissions = append(permissions, "enrollments.read")
			case "incentives": permissions = append(permissions, "incentives.read")
			case "draw-results": permissions = append(permissions, "draws.read")
			case "customer-receipts": permissions = append(permissions, "cash_receipts.read")
			case "supplier-payments": permissions = append(permissions, "supplier_payments.read")
			case "party-ledger": permissions = append(permissions, "parties.read")
			}
			return a.requireAnyPermission(permissions...)(next)(c)
		}
	}
}

func (a *API) requirePermissionOrRole(permission string, allowedRoles ...string) echo.MiddlewareFunc {
	roleSet := map[string]bool{}
	for _, role := range allowedRoles { roleSet[role] = true }
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			claims, err := a.requestClaims(c)
			if err != nil { return err }
			if roleSet[claims.Role] { return next(c) }
			allowed, err := a.service.Can(c.Request().Context(), claims.Subject, permission)
			if err != nil || !allowed { return echo.NewHTTPError(http.StatusForbidden, "insufficient permissions") }
			return next(c)
		}
	}
}

func (a *API) currentAccess(c *echo.Context) error {
	claims, err := a.requestClaims(c)
	if err != nil { return err }
	access, err := a.service.UserAccess(c.Request().Context(), claims.Subject)
	if err != nil { return echo.NewHTTPError(http.StatusUnauthorized, "account is inactive") }
	return c.JSON(http.StatusOK, access)
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (a *API) login(c *echo.Context) error {
	var req loginRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request")
	}
	id, hash, role, err := a.service.Login(c.Request().Context(), req.Username)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid username or password")
	}
	ok, err := security.VerifyPassword(hash, req.Password)
	if err != nil || !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid username or password")
	}
	access, err := a.service.UserAccess(c.Request().Context(), id)
	if err != nil { return echo.NewHTTPError(http.StatusUnauthorized, "invalid username or password") }
	minutes := 15
	if _, err := fmt.Sscan(a.config.JWTLifetimeMinutes, &minutes); err != nil || minutes < 1 || minutes > 1440 {
		minutes = 15
	}
	token, err := a.tokens.Issue(id, role)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, application.Login{UserID: id, Username: req.Username, Role: role, Permissions: access.Permissions, IsSuperAdmin: access.IsSuperAdmin, Token: token, ExpiresAt: time.Now().Add(time.Duration(minutes) * time.Minute)})
}

func (a *API) list(kind string) echo.HandlerFunc {
	return func(c *echo.Context) error {
		limit, offset := 100, 0
		var err error
		if raw := c.QueryParam("limit"); raw != "" {
			limit, err = strconv.Atoi(raw)
			if err != nil || limit < 1 || limit > 200 {
				return echo.NewHTTPError(http.StatusBadRequest, "limit must be between 1 and 200")
			}
		}
		if raw := c.QueryParam("offset"); raw != "" {
			offset, err = strconv.Atoi(raw)
			if err != nil || offset < 0 {
				return echo.NewHTTPError(http.StatusBadRequest, "offset must be zero or greater")
			}
		}
		items, err := a.service.List(c.Request().Context(), kind, limit, offset)
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, items)
	}
}
func readJSON(c *echo.Context) (json.RawMessage, error) {
	var raw json.RawMessage
	if err := c.Bind(&raw); err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "invalid JSON body")
	}
	if !json.Valid(raw) {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "invalid JSON body")
	}
	return raw, nil
}
func (a *API) create(kind string) echo.HandlerFunc {
	return func(c *echo.Context) error {
		raw, err := readJSON(c)
		if err != nil {
			return err
		}
		out, err := a.service.SaveMaster(c.Request().Context(), a.actor(c), kind, "", raw)
		if err != nil {
			return serviceError(err)
		}
		return c.JSONBlob(http.StatusCreated, out)
	}
}
func (a *API) update(kind string) echo.HandlerFunc {
	return func(c *echo.Context) error {
		raw, err := readJSON(c)
		if err != nil {
			return err
		}
		out, err := a.service.SaveMaster(c.Request().Context(), a.actor(c), kind, c.Param("id"), raw)
		if err != nil {
			return serviceError(err)
		}
		return c.JSONBlob(http.StatusOK, out)
	}
}
func (a *API) deactivate(kind string) echo.HandlerFunc {
	return func(c *echo.Context) error {
		if err := a.service.DeactivateMaster(c.Request().Context(), a.actor(c), kind, c.Param("id")); err != nil {
			return serviceError(err)
		}
		return c.NoContent(http.StatusNoContent)
	}
}

func (a *API) recordPayment(c *echo.Context) error {
	var req application.PaymentInput
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid payment")
	}
	out, err := a.service.RecordPayment(c.Request().Context(), a.actor(c), req)
	if err != nil {
		return serviceError(err)
	}
	return c.JSONBlob(http.StatusCreated, out)
}
func (a *API) recordDraw(c *echo.Context) error {
	var req application.DrawInput
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid draw result")
	}
	out, err := a.service.RecordDraw(c.Request().Context(), a.actor(c), req)
	if err != nil {
		return serviceError(err)
	}
	return c.JSONBlob(http.StatusCreated, out)
}

func (a *API) report(c *echo.Context) error {
	from, to := c.QueryParam("from"), c.QueryParam("to")
	if from == "" || to == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "from and to dates are required (YYYY-MM-DD)")
	}
	start, err := time.Parse("2006-01-02", from)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "from must use YYYY-MM-DD")
	}
	end, err := time.Parse("2006-01-02", to)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "to must use YYYY-MM-DD")
	}
	if end.Before(start) || end.Sub(start) > 3*365*24*time.Hour {
		return echo.NewHTTPError(http.StatusBadRequest, "date range must be forward and no longer than three years")
	}
	kind := c.Param("kind")
	if kind == "cashbook" || kind == "customer-receipts" || kind == "supplier-payments" || kind == "party-ledger" {
		items, err := a.service.FinancialReport(c.Request().Context(), kind, from, to, c.QueryParam("partyId"))
		if err != nil {
			return serviceError(err)
		}
		return c.JSON(http.StatusOK, items)
	}
	items, err := a.service.Report(c.Request().Context(), kind, from, to)
	if err != nil {
		return serviceError(err)
	}
	return c.JSON(http.StatusOK, items)
}

func (a *API) receipt(c *echo.Context) error {
	out, err := a.service.Receipt(c.Request().Context(), a.actor(c), c.Param("id"))
	if err != nil {
		return serviceError(err)
	}
	return c.JSONBlob(http.StatusOK, out)
}
func (a *API) createEnrollment(c *echo.Context) error {
	var req struct {
		MemberID   string `json:"memberId"`
		SchemeID   string `json:"schemeId"`
		StartMonth string `json:"startMonth"`
		EndsOn     string `json:"endsOn"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid enrollment")
	}
	out, err := a.service.CreateEnrollment(c.Request().Context(), a.actor(c), req.MemberID, req.SchemeID, req.StartMonth, req.EndsOn)
	if err != nil {
		return serviceError(err)
	}
	return c.JSONBlob(http.StatusCreated, out)
}
func (a *API) createMyCustomer(c *echo.Context) error {
	var req struct {
		Name string `json:"name"`
		Phone string `json:"phone"`
		Email string `json:"email"`
		Address string `json:"address"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.Bind(&req); err != nil { return echo.NewHTTPError(http.StatusBadRequest, "invalid customer") }
	out, err := a.service.CreateMyCustomer(c.Request().Context(), a.actor(c), req.Name, req.Phone, req.Email, req.Address, req.Username, req.Password)
	if err != nil { return serviceError(err) }
	return c.JSONBlob(http.StatusCreated, out)
}
func (a *API) setMyPaymentFrequency(c *echo.Context) error {
	var req struct { Frequency string `json:"frequency"` }
	if err := c.Bind(&req); err != nil { return echo.NewHTTPError(http.StatusBadRequest, "invalid payment frequency") }
	out, err := a.service.SetMyPaymentFrequency(c.Request().Context(), a.actor(c), c.Param("id"), req.Frequency)
	if err != nil { return serviceError(err) }
	return c.JSONBlob(http.StatusOK, out)
}
func (a *API) updateEnrollmentStatus(c *echo.Context) error {
	var req struct {
		Status string `json:"status"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid status")
	}
	out, err := a.service.UpdateEnrollmentStatus(c.Request().Context(), a.actor(c), c.Param("id"), req.Status)
	if err != nil {
		return serviceError(err)
	}
	return c.JSONBlob(http.StatusOK, out)
}
func (a *API) createIncentive(c *echo.Context) error {
	var req struct {
		PromoterID      string `json:"promoterId"`
		Period          string `json:"period"`
		AmountMinor     int64  `json:"amountMinor"`
		CommissionMinor int64  `json:"commissionMinor"`
		Currency        string `json:"currency"`
		Description     string `json:"description"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid incentive")
	}
	out, err := a.service.CreateIncentive(c.Request().Context(), a.actor(c), req.PromoterID, req.Period, req.AmountMinor, req.CommissionMinor, req.Currency, req.Description)
	if err != nil {
		return serviceError(err)
	}
	return c.JSONBlob(http.StatusCreated, out)
}
func (a *API) updateIncentiveStatus(c *echo.Context) error {
	var req struct {
		Status string `json:"status"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid status")
	}
	out, err := a.service.UpdateIncentiveStatus(c.Request().Context(), a.actor(c), c.Param("id"), req.Status)
	if err != nil {
		return serviceError(err)
	}
	return c.JSONBlob(http.StatusOK, out)
}
func (a *API) audit(c *echo.Context) error {
	limit := 100
	if value := c.QueryParam("limit"); value != "" {
		if _, err := fmt.Sscan(value, &limit); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "limit must be numeric")
		}
	}
	items, err := a.service.Audit(c.Request().Context(), limit)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, items)
}
func (a *API) myMembers(c *echo.Context) error {
	items, err := a.service.MyMembers(c.Request().Context(), a.actor(c).UserID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, items)
}
func (a *API) myMember(c *echo.Context) error {
	item, err := a.service.MyMember(c.Request().Context(), a.actor(c).UserID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "member profile not found")
	}
	return c.JSONBlob(http.StatusOK, item)
}
func (a *API) myPayments(c *echo.Context) error {
	items, err := a.service.MyPayments(c.Request().Context(), a.actor(c).UserID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, items)
}
func (a *API) paymentsMine(c *echo.Context) error {
	actor := a.actor(c)
	items, err := a.service.PaymentsFor(c.Request().Context(), actor.Role, actor.UserID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, items)
}
func (a *API) enrollments(c *echo.Context) error {
	items, err := a.service.EnrollmentsFor(c.Request().Context(), "admin", a.actor(c).UserID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, items)
}
func (a *API) enrollmentsMine(c *echo.Context) error {
	actor := a.actor(c)
	items, err := a.service.EnrollmentsFor(c.Request().Context(), actor.Role, actor.UserID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, items)
}
func (a *API) publicSchemes(c *echo.Context) error {
	items, err := a.service.PublicSchemes(c.Request().Context())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, items)
}
func (a *API) myIncentives(c *echo.Context) error {
	items, err := a.service.MyIncentives(c.Request().Context(), a.actor(c).UserID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, items)
}

func (a *API) listParties(c *echo.Context) error {
	items, err := a.service.ListParties(c.Request().Context(), c.QueryParam("type"))
	if err != nil {
		return serviceError(err)
	}
	return c.JSON(http.StatusOK, items)
}
func (a *API) createParty(c *echo.Context) error {
	raw, err := readJSON(c)
	if err != nil {
		return err
	}
	out, err := a.service.SaveParty(c.Request().Context(), a.actor(c), "", raw)
	if err != nil {
		return serviceError(err)
	}
	return c.JSONBlob(http.StatusCreated, out)
}
func (a *API) updateParty(c *echo.Context) error {
	raw, err := readJSON(c)
	if err != nil {
		return err
	}
	out, err := a.service.SaveParty(c.Request().Context(), a.actor(c), c.Param("id"), raw)
	if err != nil {
		return serviceError(err)
	}
	return c.JSONBlob(http.StatusOK, out)
}
func (a *API) deactivateParty(c *echo.Context) error {
	if err := a.service.DeactivateParty(c.Request().Context(), a.actor(c), c.Param("id")); err != nil {
		return serviceError(err)
	}
	return c.NoContent(http.StatusNoContent)
}
func (a *API) listCustomerReceipts(c *echo.Context) error {
	items, err := a.service.ListFinancialEntries(c.Request().Context(), "receipt")
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, items)
}
func (a *API) listSupplierPayments(c *echo.Context) error {
	items, err := a.service.ListFinancialEntries(c.Request().Context(), "payment")
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, items)
}
func (a *API) recordCustomerReceipt(c *echo.Context) error {
	var input application.FinancialEntryInput
	if err := c.Bind(&input); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid customer receipt")
	}
	out, err := a.service.RecordFinancialEntry(c.Request().Context(), a.actor(c), "receipt", input)
	if err != nil {
		return serviceError(err)
	}
	return c.JSONBlob(http.StatusCreated, out)
}
func (a *API) recordSupplierPayment(c *echo.Context) error {
	var input application.FinancialEntryInput
	if err := c.Bind(&input); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid supplier payment")
	}
	out, err := a.service.RecordFinancialEntry(c.Request().Context(), a.actor(c), "payment", input)
	if err != nil {
		return serviceError(err)
	}
	return c.JSONBlob(http.StatusCreated, out)
}
func (a *API) financialReceipt(c *echo.Context) error {
	out, err := a.service.FinancialReceipt(c.Request().Context(), c.Param("id"))
	if err != nil {
		return serviceError(err)
	}
	var entry struct { Direction string `json:"direction"` }
	if err := json.Unmarshal(out, &entry); err != nil { return err }
	permission := "cash_receipts.read"
	if entry.Direction == "payment" { permission = "supplier_payments.read" }
	actor := a.actor(c)
	allowed, err := a.service.Can(c.Request().Context(), actor.UserID, permission)
	if err != nil || !allowed { return echo.NewHTTPError(http.StatusForbidden, "insufficient permissions") }
	return c.JSONBlob(http.StatusOK, out)
}
