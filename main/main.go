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
		go welcome(conn) // запускаем горутину для обработки запроса
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

func welcome(conn net.Conn) {

	for {
		conn.Write([]byte("У вас имеется учетная запись? (y/n):"))
		buff := make([]byte, 1024)

		n, err := conn.Read(buff)

		if err != nil {
			break
		}
		answ := strings.ToLower(string(buff[0:n]))
		if answ == "n" {
			registration(conn)
			break
		} else if answ == "y" {
			authorization(conn)
			break
		}

	}

	//defer conn.Close() /////????? куда
	//defer s.Done()

}

func registration(conn net.Conn) {
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
	authorization(conn)
}

func authorization(conn net.Conn) {

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

			wg.Add(2)
			go sender(&wg, conn, db, login)
			go mailing(&wg, conn, db)
			_, err = db.Query("insert into chatlog values (default, 'Server', 'Пользователь " + login + " присоединился')")
			if err != nil {
				fmt.Println("Ошибка отправки сообщения в таблицу:", err)
			}
			var a, b string
			row = db.QueryRow("select login, message from chatlog order by id desc limit 1")
			err = row.Scan(&a, &b)
			if err != nil {
				fmt.Printf("Ошибка:", err)
			}

			conn.Write([]byte(a + ": " + b + "\n"))

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

func sender(wg *sync.WaitGroup, conn net.Conn, db *sql.DB, login string) { //отправка принятых сообщений в БД
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

	}

	//wg.Done()
}

func mailing(wg *sync.WaitGroup, conn net.Conn, db *sql.DB) {

	row := db.QueryRow("select id from chatlog order by id desc limit 1")
	var firstId, id int
	var login, message string

	err := row.Scan(&firstId)
	if err != nil {
		fmt.Printf("Ошибка:", err)
		_, err = db.Query("insert into chatlog values (default, ' ', ' ')")
		if err != nil {
			fmt.Println("Ошибка отправки сообщения в таблицу:", err)
		}
		fmt.Println("Вставка пустой строки в пустую таблицу")
		firstId = 1
	}
	for {
		row = db.QueryRow("select id, login, message from chatlog order by id desc limit 1")
		err = row.Scan(&id, &login, &message)
		if err != nil {
			fmt.Printf("Ошибка:", err)
		}
		if id != firstId {
			conn.Write([]byte(login + ": " + message + "\n"))
			firstId = id
		} else {
			continue
		}

	}
	//wg.Done()

}
