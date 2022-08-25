package main

import (
	"database/sql"
	"fmt"
	_ "github.com/lib/pq"
	"net"
	"strings"
	"sync"
)

var wg sync.WaitGroup

func main() {
	ch := make(chan int)

	listener, err := net.Listen("tcp", ":4545")

	if err != nil {
		fmt.Println(err)
		return
	}
	defer listener.Close()
	fmt.Println("Server is running...")

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println(err)
			conn.Close()
			continue
		}
		wg.Add(1)
		go welcome(conn, ch) // запускаем горутину для обработки запроса
	}

	wg.Wait()

}

func dbConnection() *sql.DB {
	connStr := "user=root password=123 dbname=postgres sslmode=disable"
	db, err := sql.Open("postgres", connStr)

	if err != nil {
		fmt.Printf("Ошибка подключения к БД:%s\n", err)
	}
	return db
}

func welcome(conn net.Conn, ch chan int) {

	for {
		conn.Write([]byte("У вас имеется учетная запись? (y/n):"))
		buff := make([]byte, 1024)

		n, err := conn.Read(buff)

		if err != nil {
			break
		}
		answ := strings.ToLower(string(buff[0:n]))
		if answ == "n" {
			registration(conn, ch)
			break
		} else if answ == "y" {
			authorization(conn, ch)
			break
		}

	}

	//defer conn.Close() /////????? куда
	//defer s.Done()

}

func registration(conn net.Conn, ch chan int) {
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
		if password == passwordConfirmed {
			break
		} else {
			conn.Write([]byte("Введеные пароли не совпадают. Повторите попытку\n"))
		}
	}
	db := dbConnection()

	_, err := db.Query("insert into users values ('" + login + "', '" + password + "')")
	if err != nil {
		fmt.Println("Ошибка вставки данных в таблицу:", err)
	}
	conn.Write([]byte("Регистрация прошла успешно!\n"))
	db.Close()
	authorization(conn, ch)
}

func authorization(conn net.Conn, ch chan int) {

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
			conn.Write([]byte("login " + login))
			wg.Add(1)
			go sender(&wg, conn, db, login, ch)
			break
		} else {
			conn.Write([]byte("Неверный логин/пароль\n"))
			if i == 2 {
				conn.Write([]byte("До свидания!"))
				conn.Close()
			}
		}

	}

	wg.Done()
}

func sender(wg *sync.WaitGroup, conn net.Conn, db *sql.DB, login string, ch chan int) { //отправка принятых сообщений в БД
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
		}
		/*ch <- 111

		timeout := time.After(time.Second)
		select {
		case res := <-ch:
			fmt.Println(res)
		case <-timeout:
			continue
		}*/
		mailing(conn, db)

	}

	wg.Done()
}

func mailing(conn net.Conn, db *sql.DB) {
	row := db.QueryRow("select login, message from chatlog order by id desc limit 1")

	var login, message string
	err := row.Scan(&login, &message)
	if err != nil {
		fmt.Printf("Ошибка:", err)
	}
	conn.Write([]byte(login + ": " + message + "\n"))
}
