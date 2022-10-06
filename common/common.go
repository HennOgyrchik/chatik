package common

import (
	"database/sql"
	"errors"
	"fmt"
	_ "github.com/lib/pq"
	"net"
)

type UsrInt interface {
	GetIp() string
	GetIsAuthorized() bool
	GetLogin() string
}

type UserWeb struct {
	Ip           string
	Login        string
	IsAuthorized bool
}

type UserCLI struct {
	Conn         net.Conn
	Ch           chan bool
	Login        string
	MainChan     chan string
	Ip           string
	IsAuthorized bool
}

type UsrList struct {
	Ip    string
	Login string
}

type ActiveUsers struct {
	login string
}

func (user *UserWeb) GetLogin() string {
	return user.Login
}

func (user *UserCLI) GetLogin() string {
	return user.Login
}

func (user *UserWeb) GetIp() string {
	return user.Ip
}

func (user *UserCLI) GetIp() string {
	return user.Ip
}

func (user *UserWeb) GetIsAuthorized() bool {
	return user.IsAuthorized
}

func (user *UserCLI) GetIsAuthorized() bool {
	return user.IsAuthorized
}

var MapUsers = make(map[string]UsrInt)

var ListUser = []string{}

func AddUserToMap(usrInt UsrInt) {
	MapUsers[usrInt.GetIp()] = usrInt
	fmt.Println(MapUsers[usrInt.GetIp()])
}

func DbConnection() (*sql.DB, error) {
	connStr := "user=root password=123 dbname=postgres sslmode=disable"
	db, err := sql.Open("postgres", connStr)

	if err != nil {
		return nil, err
	}
	return db, nil
}

func Registration(login string, password string) error {
	db, err := DbConnection()
	defer db.Close()
	if err != nil {
		return err
	}
	row := db.QueryRow("select count(*) from users where login = '" + login + "'")

	var count byte
	err = row.Scan(&count)
	if err != nil {
		return err
	}
	if count != 0 {
		return errors.New("Пользователь с таким логином уже существует\n")
	}

	_, err = db.Query("insert into users values ('" + login + "', '" + password + "')")
	if err != nil {
		return err
	}

	return nil
}

func Authorization(login string, password string) error {
	db, err := DbConnection()
	if err != nil {
		fmt.Println(err)
		return err
	}
	defer db.Close()

	row := db.QueryRow("select login, pass from users where login='" + login + "'")

	var loginFromDb, passwordFromDb string
	err = row.Scan(&loginFromDb, &passwordFromDb)
	if err != nil {
		return errors.New("Неверный логин/пароль\n")
	}

	if login == loginFromDb && password == passwordFromDb {
		_, err = db.Query("insert into chatlog values (default, 'Server', 'Пользователь " + login + " присоединился')")
		if err != nil {
			return errors.New("Ошибка записи данных в таблицу\n")
		}
		/*for _, value := range ListUser {
			if value == login || value == "" {
				fmt.Println("Fwefcaf")
				return errors.New("Необходимо авторизоваться")
			}
		}*/
		ListUser = append(ListUser, login)
		fmt.Println(ListUser)
		return nil
	} else {
		return errors.New("Неверный логин/пароль\n")

	}

}

func (user *UserCLI) Reader() (string, error) {
	buff := make([]byte, 1024)
	n, err := user.Conn.Read(buff)
	if err != nil {
		user.MainChan <- user.Ip
		return "", err
	}
	return string(buff[0:n]), nil
}

func (user *UserCLI) Writer(s string) {
	user.Conn.Write([]byte(s))
}

func (user *UserCLI) SenderCLI(db *sql.DB) {
	for {
		message, err := user.Reader()
		if err != nil {
			break
		}
		if len(message) != 0 {
			_, err := db.Query("insert into chatlog values (default, '" + user.Login + "', '" + message + "')")
			if err != nil {
				fmt.Println("Ошибка отправки сообщения в таблицу:", err)
				continue
			}
			user.MainChan <- "send"
		} else {
			continue
		}
	}
	fmt.Println("sender закрыт")
	defer db.Close()
}

func (user *UserCLI) Mailing(db *sql.DB) {
	var login, message string
	for {
		val, _ := <-user.Ch
		if val {
			row := db.QueryRow("select login, message from chatlog order by id desc limit 1")
			err := row.Scan(&login, &message)
			if err != nil {
				fmt.Printf("Ошибка:", err)
				break
			}
			user.Writer(login + ": " + message + "\n")

		} else {
			break
		}
	}
	fmt.Println("mailing закрыт")
}

func MainChanListener(mainChan chan string) {
	/*for {
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
	}*/
}
