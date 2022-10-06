package handler

import (
	"chatik/common"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/labstack/echo/v4"
	"net/http"
)

var usrWeb common.UserWeb

func HomeHandler(c echo.Context) error {
	//qwe:=make(map[string]usr)
	return c.Render(http.StatusOK, "login.html", map[string]interface{}{})
}

func RegistrationPage(c echo.Context) error {
	return c.Render(http.StatusOK, "registration.html", map[string]interface{}{})
}

func ChatPage(c echo.Context) error {
	return c.Render(http.StatusOK, "chat.html", map[string]interface{}{})
}

func UsrList(c echo.Context) error {
	b, err := json.Marshal(common.ListUser)
	if err != nil {
		fmt.Println("JSON: ", err)

	}
	fmt.Println(string(b))
	return c.String(200, string(b))
}

func Authorization(c echo.Context) error {
	login := c.FormValue("login")
	password := c.FormValue("password")
	err := common.Authorization(login, password)
	if err != nil {
		c.Render(http.StatusOK, "login.html", map[string]interface{}{"err": err.Error()})
		return err
	}
	usrWeb.IsAuthorized = true
	usrWeb.Login = login
	usrWeb.Ip = c.Request().RemoteAddr
	common.AddUserToMap(&usrWeb)
	return ChatPage(c)
}

func Registration(c echo.Context) error {
	var err error
	login := c.FormValue("login")
	password := c.FormValue("password")
	confPass := c.FormValue("passwordConfirmed")
	if password != confPass {
		err = errors.New("Введеные пароли не совпадают!")
		c.Render(http.StatusOK, "registration.html", map[string]interface{}{"err": err.Error()})
		return err

	}
	err = common.Registration(login, password)
	if err != nil {
		c.Render(http.StatusOK, "registration.html", map[string]interface{}{"err": err.Error()})
		return err
	}
	return ChatPage(c)
}
