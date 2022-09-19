package main

import (
	"database/sql"
	"fmt"
	_ "github.com/lib/pq"
	"net"
	"strings"
	"sync"
)

var mapConn map[string]chan bool

func main() {
	var wg sync.WaitGroup
	listener, err := net.Listen("tcp", ":4545")

	if err != nil {
		fmt.Println(err)
		return
	}
	defer listener.Close()
	fmt.Println("Server is running...")

	mapConn = make(map[string]chan bool)
	mainChan := make(chan string)
	go mainChanListener(mainChan)

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println(err)
			conn.Close()
			continue
		}

		ch := make(chan bool)
		wg.Add(1)

		go welcome(conn, ch, mainChan, &wg)

		mapConn[conn.RemoteAddr().String()] = ch

	}

	wg.Wait()

}

//управление рассылкой последнего сообщения всем пользователям
func mainChanListener(mainChan chan string) {
	for {

		switch val := <-mainChan; {
		case val == "send":
			{
				for _, channel := range mapConn {
					channel <- true
				}
			}
		default: //По идее тут должны быть только сокеты: 192.168.0.3:12312 удалить пользователя из рассылки
			{
				i, _ := mapConn[val]
				close(i)
				delete(mapConn, val)
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

//приветственное сообщение, регистрация или авторизация
func welcome(conn net.Conn, ch chan bool, mainChan chan string, wg *sync.WaitGroup) {
	db := dbConnection()
	row := db.QueryRow("select count(*) from users")
	db.Close()
	var count byte
	err := row.Scan(&count)
	if err != nil {
		fmt.Printf("Ошибка:", err)
	}
	if count == 0 {
		registration(conn, ch, mainChan, wg)
	} else {

		for {
			conn.Write([]byte("У вас имеется учетная запись? (y/n):"))
			buff := make([]byte, 1024)

			n, err := conn.Read(buff)

			if err != nil {
				break
			}
			answ := strings.ToLower(string(buff[0:n]))
			if answ == "n" {
				registration(conn, ch, mainChan, wg)
				break
			} else if answ == "y" {
				authorization(conn, ch, mainChan, wg)
				break
			}

		}
	}
	fmt.Println("Закрыт welcome")
}

//регистрация нового пользователя
func registration(conn net.Conn, ch chan bool, mainChan chan string, wg *sync.WaitGroup) {
	var login, password, passwordConfirmed string
	for {
		conn.Write([]byte("Придумайте никнейм:"))
		for {
			buff := make([]byte, 1024)

			n, err := conn.Read(buff)
			if err != nil {
				break
			}
			login = string(buff[0:n])
			break
		}
		conn.Write([]byte("Придумайте пароль:"))
		for {
			buff := make([]byte, 1024)

			n, err := conn.Read(buff)
			if err != nil {
				break
			}
			password = string(buff[0:n])
			break
		}
		conn.Write([]byte("Повторите пароль:"))
		for {
			buff := make([]byte, 1024)

			n, err := conn.Read(buff)
			if err != nil {
				break
			}
			passwordConfirmed = string(buff[0:n])
			break
		}
		if password != passwordConfirmed {
			conn.Write([]byte("Введеные пароли не совпадают. Повторите попытку\n"))
			continue
		}
		db := dbConnection()
		row := db.QueryRow("select count(*) from users where login = '" + login + "'")
		db.Close()
		var count byte
		err := row.Scan(&count)
		if err != nil {
			fmt.Printf("Ошибка:", err)
		}
		if count != 0 {
			conn.Write([]byte("Пользователь с таким логином уже существует\n"))
			continue
		} else {
			break
		}
	}
	db := dbConnection()
	_, err := db.Query("insert into users values ('" + login + "', '" + password + "')")
	if err != nil {
		fmt.Println("Ошибка вставки данных в таблицу:", err)
	}
	conn.Write([]byte("Регистрация прошла успешно!\n"))
	db.Close()
	authorization(conn, ch, mainChan, wg)
	fmt.Println("Закрыт registration")
}

//авторизация пользователя
func authorization(conn net.Conn, ch chan bool, mainChan chan string, wg *sync.WaitGroup) {

	db := dbConnection()

	for i := 0; i < 3; i++ {
		var login, password string
		conn.Write([]byte("Введите логин:"))
		for {
			buff := make([]byte, 1024)

			n, err := conn.Read(buff)
			if err != nil {
				break
			}
			login = string(buff[0:n])
			break
		}
		conn.Write([]byte("Введите пароль:"))
		for {
			buff := make([]byte, 1024)

			n, err := conn.Read(buff)
			if err != nil {
				break
			}
			password = string(buff[0:n])
			break
		}

		row := db.QueryRow("select login, pass from users where login='" + login + "'")

		var loginFromDb, passwordFromDb string
		err := row.Scan(&loginFromDb, &passwordFromDb)
		if err != nil {
			fmt.Printf("Ошибка:", err)
		}

		if login == loginFromDb && password == passwordFromDb {

			wg.Add(1) // +1 добавлялся в начале

			go sender(wg, conn, db, login, mainChan)
			go mailing(wg, conn, db, ch)
			_, err = db.Query("insert into chatlog values (default, 'Server', 'Пользователь " + login + " присоединился')")
			if err != nil {
				fmt.Println("Ошибка отправки сообщения в таблицу:", err)
			}
			mainChan <- "send"
			break
		} else {
			conn.Write([]byte("Неверный логин/пароль\n"))
			if i == 2 {
				conn.Write([]byte("До свидания!"))
				conn.Close()
			}
		}

	}
	fmt.Println("Закрыт authorization")
}

//отправка принятых сообщений в БД
func sender(wg *sync.WaitGroup, conn net.Conn, db *sql.DB, login string, mainChan chan string) {
	for {
		buff := make([]byte, 1024)

		n, err := conn.Read(buff)
		if err != nil {
			break
		}
		if len(buff) != 0 {
			_, err = db.Query("insert into chatlog values (default, '" + login + "', '" + string(buff[0:n]) + "')")
			if err != nil {
				fmt.Println("Ошибка отправки сообщения в таблицу:", err)
			}
			mainChan <- "send"
		}

	}
	mainChan <- conn.RemoteAddr().String()
	fmt.Println("Закрыт sender")
	defer wg.Done()
	defer db.Close()
	defer conn.Close()
}

//отправка последнего сообщения клиенту
func mailing(wg *sync.WaitGroup, conn net.Conn, db *sql.DB, ch chan bool) {
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
			conn.Write([]byte(login + ": " + message + "\n"))
		}
	}
	fmt.Println("Закрыт mailing")
	defer wg.Done()
	defer db.Close()
	defer conn.Close()
	defer close(ch)
}
