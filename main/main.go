package main

import (
	"database/sql"
	"fmt"
	_ "github.com/lib/pq"
	"net"
	_ "runtime"
	"strings"
)

type users struct {
	conn         net.Conn
	ch           chan bool
	login        string
	mainChan     chan string
	ip           string
	isAuthorized bool
}

func main() {
	listener, err := net.Listen("tcp", ":4545")

	if err != nil {
		fmt.Println(err)
		return
	}
	defer listener.Close()
	fmt.Println("Server is running...")

	mainChan := make(chan string)
	mapUsers := make(map[string]*users)

	go mainChanListener(mapUsers, mainChan)
	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println(err)
			conn.Close()
			continue
		}
		addUserToMap(mapUsers, mainChan, conn)

		//fmt.Println("main 2 -NumGoroutine - ", runtime.NumGoroutine())
	}
}

//добавление нового пользователя в mapUser
func addUserToMap(mapUsers map[string]*users, mainChan chan string, conn net.Conn) {
	ch := make(chan bool)
	user := users{conn: conn, mainChan: mainChan, ch: ch, ip: conn.RemoteAddr().String(), isAuthorized: false}
	mapUsers[user.ip] = &user
	go welcome(&user)
}

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
}

//подключение к БД
func dbConnection() *sql.DB {
	connStr := "user=root password=123 dbname=postgres sslmode=disable"
	db, err := sql.Open("postgres", connStr)

	if err != nil {
		fmt.Printf("Ошибка подключения к БД:%s\n", err)
	}
	return db
}

//функция чтения
func reader(user *users) (string, error) {
	buff := make([]byte, 1024)
	n, err := user.conn.Read(buff)
	if err != nil {
		user.mainChan <- user.ip
		return "", err
	}
	return string(buff[0:n]), nil
}

//функция записи
func writer(conn net.Conn, s string) {
	conn.Write([]byte(s))
}

//приветственное сообщение, регистрация или авторизация
func welcome(user *users) {
	db := dbConnection()
	row := db.QueryRow("select count(*) from users")
	db.Close()
	var count int
	err := row.Scan(&count)
	if err != nil {
		fmt.Printf("Ошибка:", err)
	}
	if count == 0 {
		registration(user)
	} else {
		for {
			writer(user.conn, "У вас имеется учетная запись? (y/n):")
			val, err := reader(user)
			if err != nil {
				break
			}
			switch strings.ToLower(val) {
			case "n":
				if registration(user) != nil {
					return
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

	}
	fmt.Println("welcome закрыт")
	//fmt.Println("Welcome - NumGoroutine - ", runtime.NumGoroutine())
}

//регистрация нового пользователя
func registration(user *users) error {
	var login, password, passwordConfirmed string
	var err error
	ok := false
	for {
		writer(user.conn, "Придумайте никнейм:")
		login, err = reader(user)
		if err != nil {
			break
		}
		writer(user.conn, "Придумайте пароль:")
		password, err = reader(user)
		if err != nil {
			break
		}
		writer(user.conn, "Повторите пароль:")
		passwordConfirmed, err = reader(user)
		if err != nil {
			break
		}
		if password != passwordConfirmed {
			writer(user.conn, "Введеные пароли не совпадают. Повторите попытку\n")
			continue
		}
		db := dbConnection()
		row := db.QueryRow("select count(*) from users where login = '" + login + "'")
		db.Close()
		var count byte
		err = row.Scan(&count)
		if err != nil {
			fmt.Printf("Ошибка:", err)
		}
		if count != 0 {
			writer(user.conn, "Пользователь с таким логином уже существует\n")
			continue
		} else {
			ok = true
			break
		}
	}

	if ok {
		db := dbConnection()
		_, err = db.Query("insert into users values ('" + login + "', '" + password + "')")
		if err != nil {
			fmt.Println("Ошибка вставки данных в таблицу:", err)
		}
		writer(user.conn, "Регистрация прошла успешно!\n")
		db.Close()
		return nil
	} else {
		return err
	}
}

//авторизация пользователя
func authorization(user *users) {
	db := dbConnection()
	for i := 0; i < 3; i++ {

		writer(user.conn, "Введите логин:")
		var err error
		user.login, err = reader(user)
		if err != nil {
			break
		}
		writer(user.conn, "Введите пароль:")
		password, err := reader(user)
		if err != nil {
			break
		}

		row := db.QueryRow("select login, pass from users where login='" + user.login + "'")

		var loginFromDb, passwordFromDb string
		err = row.Scan(&loginFromDb, &passwordFromDb)
		if err != nil {
			fmt.Printf("Ошибка:", err)
		}

		if user.login == loginFromDb && password == passwordFromDb {
			user.isAuthorized = true
			go user.sender(db)
			go user.mailing(db)
			_, err = db.Query("insert into chatlog values (default, 'Server', 'Пользователь " + user.login + " присоединился')")
			if err != nil {
				fmt.Println("Ошибка отправки сообщения в таблицу:", err)
			}
			user.mainChan <- "send"
			break
		} else {
			writer(user.conn, "Неверный логин/пароль\n")
			if i == 2 {
				writer(user.conn, "До свидания!")
				user.conn.Close()
			}
		}

	}
	fmt.Println("authorization закрыт")
	//fmt.Println("authorization - NumGoroutine - ", runtime.NumGoroutine())
}

//отправка принятых сообщений в БД
func (u users) sender(db *sql.DB) {
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
}
