package httpapi

import (
	"net/http"

	"github.com/labstack/echo/v5"
	"schemeapp/backend/internal/application"
)

func (a *API) listAccounts(c *echo.Context) error {
	items, err := a.service.ListAccounts(c.Request().Context())
	if err != nil { return err }
	return c.JSON(http.StatusOK, items)
}

func (a *API) createAccount(c *echo.Context) error {
	var input application.AccountInput
	if err := c.Bind(&input); err != nil { return echo.NewHTTPError(http.StatusBadRequest, "invalid account") }
	item, err := a.service.SaveAccount(c.Request().Context(), a.actor(c), "", input)
	if err != nil { return serviceError(err) }
	return c.JSON(http.StatusCreated, item)
}

func (a *API) updateAccount(c *echo.Context) error {
	var input application.AccountInput
	if err := c.Bind(&input); err != nil { return echo.NewHTTPError(http.StatusBadRequest, "invalid account") }
	item, err := a.service.SaveAccount(c.Request().Context(), a.actor(c), c.Param("id"), input)
	if err != nil { return serviceError(err) }
	return c.JSON(http.StatusOK, item)
}

func (a *API) deleteAccount(c *echo.Context) error {
	if err := a.service.DeleteAccount(c.Request().Context(), a.actor(c), c.Param("id")); err != nil { return serviceError(err) }
	return c.NoContent(http.StatusNoContent)
}

func (a *API) listRoles(c *echo.Context) error {
	items, err := a.service.ListCompanyRoles(c.Request().Context())
	if err != nil { return err }
	return c.JSON(http.StatusOK, items)
}

func (a *API) createRole(c *echo.Context) error {
	var input application.RoleInput
	if err := c.Bind(&input); err != nil { return echo.NewHTTPError(http.StatusBadRequest, "invalid role") }
	item, err := a.service.SaveCompanyRole(c.Request().Context(), a.actor(c), "", input)
	if err != nil { return serviceError(err) }
	return c.JSON(http.StatusCreated, item)
}

func (a *API) updateRole(c *echo.Context) error {
	var input application.RoleInput
	if err := c.Bind(&input); err != nil { return echo.NewHTTPError(http.StatusBadRequest, "invalid role") }
	item, err := a.service.SaveCompanyRole(c.Request().Context(), a.actor(c), c.Param("id"), input)
	if err != nil { return serviceError(err) }
	return c.JSON(http.StatusOK, item)
}

func (a *API) deleteRole(c *echo.Context) error {
	if err := a.service.DeleteCompanyRole(c.Request().Context(), a.actor(c), c.Param("id")); err != nil { return serviceError(err) }
	return c.NoContent(http.StatusNoContent)
}

func (a *API) permissions(c *echo.Context) error {
	return c.JSON(http.StatusOK, application.Permissions())
}
