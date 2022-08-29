package main

import (
	"database/sql"
	"fmt"
	_ "github.com/lib/pq"
	"net"
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

	var chanList []chan bool
	mainChan := make(chan byte)
	go mainChanListener(mainChan, chanList, &wg)

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
		chanList = append(chanList, ch)
		wg.Add(1)
		mainChan <- 1
		go mainChanListener(mainChan, chanList, &wg)
	}

	wg.Wait()

}

func mainChanListener(mainChan chan byte, list []chan bool, wg *sync.WaitGroup) {
	for {
		val := <-mainChan
		if val == 1 {
			break
		} else if val == 2 {
			for i := 0; i < len(list); i++ {
				list[i] <- true
			}
		}
	}
	defer wg.Done()
}

func dbConnection() *sql.DB {
	connStr := "user=root password=123 dbname=postgres sslmode=disable"
	db, err := sql.Open("postgres", connStr)

	if err != nil {
		fmt.Printf("Ошибка подключения к БД:%s\n", err)
	}
	return db
}

func welcome(conn net.Conn, ch chan bool, mainChan chan byte, wg *sync.WaitGroup) {
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

}

func registration(conn net.Conn, ch chan bool, mainChan chan byte, wg *sync.WaitGroup) {
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
}

func authorization(conn net.Conn, ch chan bool, mainChan chan byte, wg *sync.WaitGroup) {

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

			go sender(wg, conn, db, login, mainChan)
			go mailing(wg, conn, db, ch)
			_, err = db.Query("insert into chatlog values (default, 'Server', 'Пользователь " + login + " присоединился')")
			if err != nil {
				fmt.Println("Ошибка отправки сообщения в таблицу:", err)
			}
			mainChan <- 2
			break
		} else {
			conn.Write([]byte("Неверный логин/пароль\n"))
			if i == 2 {
				conn.Write([]byte("До свидания!"))
				conn.Close()
			}
		}

	}

}

func sender(wg *sync.WaitGroup, conn net.Conn, db *sql.DB, login string, mainChan chan byte) { //отправка принятых сообщений в БД
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
			mainChan <- 2
		}

	}

	defer wg.Done()
	defer db.Close()
	defer conn.Close()
}

func mailing(wg *sync.WaitGroup, conn net.Conn, db *sql.DB, ch chan bool) {
	var login, message string
	for {

		if <-ch {
			row := db.QueryRow("select login, message from chatlog order by id desc limit 1")
			err := row.Scan(&login, &message)
			if err != nil {
				fmt.Printf("Ошибка:", err)
			}
			conn.Write([]byte(login + ": " + message + "\n"))
		}
	}
	defer wg.Done()
	defer db.Close()
	defer conn.Close()
	defer close(ch)
}
