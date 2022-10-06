package main

import (
	"chatik/common"
	"chatik/handler"
	"fmt"
	"github.com/labstack/echo/v4"
	"html/template"
	"io"
	"net"
	_ "runtime"
	"strings"
	"sync"
)

type users struct {
	conn         net.Conn
	ch           chan bool
	login        string
	mainChan     chan string
	ip           string
	isAuthorized bool
}

type TemplateRegistry struct {
	templates *template.Template
}

func main() {
	var wg sync.WaitGroup
	wg.Add(2)
	go httpServ()
	go cliServ()
	wg.Wait()
}

func cliServ() {
	listener, err := net.Listen("tcp", ":4545")

	if err != nil {
		fmt.Println(err)
		return
	}
	defer listener.Close()
	db, err := common.DbConnection()
	if err != nil {
		fmt.Println("Ошибка подключения к БД: ", err)
		return
	}
	_, err = db.Query("insert into chatlog values (default, 'Server', 'Server is running..')")
	if err != nil {
		fmt.Println("Ошибка отправки сообщения в таблицу:", err)
	}
	db.Close()
	fmt.Println("Server is running...")

	mainChan := make(chan string)
	//mapUsers := make(map[string]*users)

	go common.MainChanListener(mainChan)

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println(err)
			conn.Close()
			continue
		}

		ch := make(chan bool)
		user := common.UserCLI{Conn: conn, MainChan: mainChan, Ch: ch, Ip: conn.RemoteAddr().String(), IsAuthorized: false}
		common.AddUserToMap(&user)
		go welcome(&user)
		//fmt.Println("main 2 -NumGoroutine - ", runtime.NumGoroutine())
	}
}

func (t *TemplateRegistry) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.templates.ExecuteTemplate(w, name, data)
}

func httpServ() {
	e := echo.New()

	// Instantiate a template registry and register all html files inside the view folder
	e.Renderer = &TemplateRegistry{
		templates: template.Must(template.ParseGlob("view/*.html")),
	}

	// Route => handler
	e.GET("/", handler.HomeHandler)
	e.POST("/", handler.Authorization)
	e.GET("/registration", handler.RegistrationPage)
	e.POST("/registration", handler.Registration)
	e.POST("/chat", handler.ChatPage)
	e.GET("/chat", handler.UsrList)

	// Start the Echo server
	e.Logger.Fatal(e.Start(":1010"))
}

/*
func mainChanListener(mapUsers map[string]*users, mainChan chan string) {
	for {
		val := <-mainChan
		switch val {
		case "send": //рассылка нового сообщения всем из mapUser
			{
				for _, user := range mapUsers {
					if user.isAuthorized {
						user.ch <- true
					} else {
						continue
					}
				}
			}
		default: //удаление user из mapUser по ip
			{
				user, _ := mapUsers[val]
				user.ch <- false
				close(user.ch)
				delete(mapUsers, val)
			}

		}
	}
}*/

/*
//подключение к БД
func dbConnection() *sql.DB {
	connStr := "user=root password=123 dbname=postgres sslmode=disable"
	db, err := sql.Open("postgres", connStr)

	if err != nil {
		fmt.Printf("Ошибка подключения к БД:%s\n", err)
	}
	return db
}*/

//приветственное сообщение, регистрация или авторизация
func welcome(user *common.UserCLI) {
	for {
		user.Writer("У вас имеется учетная запись? (y/n):")
		val, err := user.Reader()
		if err != nil {
			break
		}
		switch strings.ToLower(val) {
		case "n":
			{
				if getUserPropertyForRegistration(user) != nil {
					return
				}
			}
		case "y":
		case "":
			return
		default:
			continue
		}
		authorization(user)
		break
	}
	fmt.Println("welcome закрыт")
	//fmt.Println("Welcome - NumGoroutine - ", runtime.NumGoroutine())
}

func getUserPropertyForRegistration(user *common.UserCLI) error {
	var login, password, passwordConfirmed string
	var err error

	for {
		user.Writer("Придумайте никнейм:")
		login, err = user.Reader()
		if err != nil {
			break
		}
		user.Writer("Придумайте пароль:")
		password, err = user.Reader()
		if err != nil {
			break
		}
		user.Writer("Повторите пароль:")
		passwordConfirmed, err = user.Reader()
		if err != nil {
			break
		}
		if password != passwordConfirmed {
			user.Writer("Введеные пароли не совпадают. Повторите попытку\n")
			continue
		}
		err = common.Registration(login, password)
		if err != nil {
			user.Writer(err.Error())
		} else {
			user.Writer("Регистрация прошла успешно!\n")
			break
		}

	}
	return err
}

//авторизация пользователя
func authorization(user *common.UserCLI) {
	for i := 0; i < 3; i++ {

		user.Writer("Введите логин:")
		var err error
		user.Login, err = user.Reader()
		if err != nil {
			break
		}
		user.Writer("Введите пароль:")
		password, err := user.Reader()
		if err != nil {
			break
		}
		err = common.Authorization(user.Login, password)
		if err != nil {
			if i == 2 {
				user.Writer("Пока!")
				user.Conn.Close()
				break
			}
			user.Writer(err.Error())
			continue
		}

		user.IsAuthorized = true
		db, err := common.DbConnection()
		if err != nil {
			fmt.Println(err)
			break
		}
		go user.SenderCLI(db)
		go user.Mailing(db)

		user.MainChan <- "send"
		break

	}
	fmt.Println("authorization закрыт")
	//fmt.Println("authorization - NumGoroutine - ", runtime.NumGoroutine())
}

/*
//отправка принятых сообщений в БД
func (u common.UserCLI) sender(db *sql.DB) {
	for {
		message, err := reader(&u)
		if err != nil {
			break
		}
		if len(message) != 0 {
			_, err := db.Query("insert into chatlog values (default, '" + u.login + "', '" + message + "')")
			if err != nil {
				fmt.Println("Ошибка отправки сообщения в таблицу:", err)
				continue
			}
			u.mainChan <- "send"
		} else {
			continue
		}
	}
	fmt.Println("sender закрыт")
	defer db.Close()
}
/*

//отправка последнего (нового) сообщения клиенту
func (u users) mailing(db *sql.DB) {
	var login, message string
	for {
		val, _ := <-u.ch
		if val {
			row := db.QueryRow("select login, message from chatlog order by id desc limit 1")
			err := row.Scan(&login, &message)
			if err != nil {
				fmt.Printf("Ошибка:", err)
				break
			}
			writer(u.conn, login+": "+message+"\n")

		} else {
			break
		}
	}
	fmt.Println("mailing закрыт")
}*/
