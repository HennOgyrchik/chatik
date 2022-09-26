package main

import (
	"database/sql"
	"fmt"
	_ "github.com/lib/pq"
	"net"
	"runtime"
	"strings"
	"sync"
)

func main() {
	var wg sync.WaitGroup
	listener, err := net.Listen("tcp", ":4545")

	if err != nil {
		fmt.Println(err)
		return
	}
	defer listener.Close()
	fmt.Println("Server is running...")

	mainChan := make(chan string)
	listChan := make(chan net.Conn)
	wg.Add(1)
	go mainChanListener(mainChan, listChan, &wg)
	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println(err)
			conn.Close()
			continue
		}
		listChan <- conn

		fmt.Println("main 2 -NumGoroutine - ", runtime.NumGoroutine())

	}
	wg.Wait()

}

//управление рассылкой последнего сообщения всем пользователям
func mainChanListener(mainChan chan string, listChan chan net.Conn, wg *sync.WaitGroup) {
	mapConn := make(map[string]chan bool)
	for {
		select {
		case conn := <-listChan:
			{
				ch := make(chan bool)
				mapConn[conn.RemoteAddr().String()] = ch
				go welcome(conn, ch, mainChan)
			}
		case val := <-mainChan:
			{
				switch val {
				case "send":
					{
						for _, channel := range mapConn {
							channel <- true
						}
					}
				default: //По идее тут должны быть только сокеты: 192.168.0.3:12312 удалить пользователя из рассылки
					{
						i, ok := mapConn[val]
						fmt.Println(ok)
						close(i)
						delete(mapConn, val)
					}

				}
			}
		}

	}
	defer wg.Done()
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

//приветственное сообщение, регистрация или авторизация
func welcome(conn net.Conn, ch chan bool, mainChan chan string) {
	db := dbConnection()
	row := db.QueryRow("select count(*) from users")
	db.Close()
	var count int
	err := row.Scan(&count)
	if err != nil {
		fmt.Printf("Ошибка:", err)
	}
	if count == 0 {
		registration(conn, mainChan)
	} else {
		for {
			writer(conn, "У вас имеется учетная запись? (y/n):")
			val, err := reader(conn, mainChan)
			if err != nil {
				break
			}
			switch strings.ToLower(val) {
			case "n":
				{
					if registration(conn, mainChan) != nil {
						break
					}
					authorization(conn, ch, mainChan)
					break
				}
			case "y":
				{
					authorization(conn, ch, mainChan)
					break
				}
			case "":
				{
					break
				}

			default:
				continue
			}
			break
		}

	}
	fmt.Println("Welcome - NumGoroutine - ", runtime.NumGoroutine())
}

func reader(conn net.Conn, mainChan chan string) (string, error) {
	buff := make([]byte, 1024)
	n, err := conn.Read(buff)
	if err != nil {
		mainChan <- conn.RemoteAddr().String()
		return "", err
	}
	//fmt.Println(buff[n:n])
	return string(buff[0:n]), nil
}

//регистрация нового пользователя
func registration(conn net.Conn, mainChan chan string) error {
	var login, password, passwordConfirmed string
	var err error
	ok := false
	for {
		writer(conn, "Придумайте никнейм:")
		login, err = reader(conn, mainChan)
		if err != nil {
			break
		}
		writer(conn, "Придумайте пароль:")
		password, err = reader(conn, mainChan)
		if err != nil {
			break
		}
		writer(conn, "Повторите пароль:")
		passwordConfirmed, err = reader(conn, mainChan)
		if err != nil {
			break
		}
		if password != passwordConfirmed {
			writer(conn, "Введеные пароли не совпадают. Повторите попытку\n")
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
			writer(conn, "Пользователь с таким логином уже существует\n")
			continue
		} else {
			ok = true
			break
		}
	}
	fmt.Println("aesdasd:", ok)
	if ok {
		db := dbConnection()
		_, err = db.Query("insert into users values ('" + login + "', '" + password + "')")
		if err != nil {
			fmt.Println("Ошибка вставки данных в таблицу:", err)
		}
		writer(conn, "Регистрация прошла успешно!\n")
		db.Close()
		return nil
	} else {
		return err
	}
}

func writer(conn net.Conn, s string) {
	conn.Write([]byte(s))
}

//авторизация пользователя
func authorization(conn net.Conn, ch chan bool, mainChan chan string) {
	db := dbConnection()
	for i := 0; i < 3; i++ {

		writer(conn, "Введите логин:")
		login, err := reader(conn, mainChan)
		if err != nil {
			break
		}
		writer(conn, "Введите пароль:")
		password, err := reader(conn, mainChan)
		if err != nil {
			break
		}

		row := db.QueryRow("select login, pass from users where login='" + login + "'")

		var loginFromDb, passwordFromDb string
		err = row.Scan(&loginFromDb, &passwordFromDb)
		if err != nil {
			fmt.Printf("Ошибка:", err)
		}

		if login == loginFromDb && password == passwordFromDb {
			go sender(conn, db, login, mainChan)
			go mailing(conn, db, ch)
			_, err = db.Query("insert into chatlog values (default, 'Server', 'Пользователь " + login + " присоединился')")
			if err != nil {
				fmt.Println("Ошибка отправки сообщения в таблицу:", err)
			}
			mainChan <- "send"
			break
		} else {
			writer(conn, "Неверный логин/пароль\n")
			if i == 2 {
				writer(conn, "До свидания!")
				conn.Close()
			}
		}

	}
	fmt.Println("authorization - NumGoroutine - ", runtime.NumGoroutine())
}

//отправка принятых сообщений в БД
func sender(conn net.Conn, db *sql.DB, login string, mainChan chan string) {
	for {
		message, err := reader(conn, mainChan)
		if err != nil {
			break
		}
		if len(message) != 0 {
			_, err := db.Query("insert into chatlog values (default, '" + login + "', '" + message + "')")
			if err != nil {
				fmt.Println("Ошибка отправки сообщения в таблицу:", err)
			}
			mainChan <- "send"
		} else {
			break
		}
	}
	//fmt.Println("sender - NumGoroutine - ", runtime.NumGoroutine())
	defer conn.Close()
	defer db.Close()
}

//отправка последнего сообщения клиенту
func mailing(conn net.Conn, db *sql.DB, ch chan bool) {
	var login, message string
	for {
		_, ok := <-ch
		if ok {
			row := db.QueryRow("select login, message from chatlog order by id desc limit 1")
			err := row.Scan(&login, &message)
			if err != nil {
				fmt.Printf("Ошибка:", err)
				break
			}
			writer(conn, login+": "+message+"\n")

		} else {
			break
		}
	}
	//fmt.Println("mailing - NumGoroutine - ", runtime.NumGoroutine())
	defer conn.Close()
	defer db.Close()
}
