package main

import (
	"net"
	"os"
	"fmt"
	"flag"
	"log"
	"bufio"
	"strconv"
	"strings"
	"sync/atomic"
	"runtime"
)

const(
	CONN_TYPE = "tcp"
	BREAK = "Break"
	GETREQUAST = "Get"
	POSTREQUAST = "Post"
	NAMEREQUAST = "Name"
	AMOUNTOFCONN = "Amount Of Connections"
	COMMANDERROR = ""
	BREAKCHARACTER = ":"
	ONLINE = "Online"
	MESSAGE = "Message"
	MESSAGEBREAKCHAR = "|"
)

type Client struct{
	Name string
	Conn net.Conn
}

func (c Client)SetName(name string){
	c.Name = name
}

var amountOfConnectionsToServer int64
//var clients = make([]net.Conn,0)
var clients = make(map[string]Client)

func main(){
	switch{
	case len(os.Args) <= 2:
		fmt.Fprintf(os.Stderr,"Usage: %s workflow,port,core(optional)\n",os.Args[0])
		os.Exit(1)
	case len(os.Args) > 4:
		fmt.Fprintf(os.Stderr,"%s too many arguments",os.Args[0])
		os.Exit(1)
	}

	workflowPtr := flag.String("workflow","crash","a string")
	portPtr := flag.String("port","8080","a string")
	amountOfCorePtr := flag.Int("core",1,"an int")

	flag.Parse()

	fmt.Println("workflow="+*workflowPtr + " port:"+*portPtr)
	switch *workflowPtr{
		case "server":
			runtime.GOMAXPROCS(*amountOfCorePtr)
			masterSocketListener, err := net.Listen(CONN_TYPE,":"+*portPtr)
			if err != nil{
				fmt.Println("Error while listening master socket:",err.Error())
				os.Exit(1)
			}

			defer masterSocketListener.Close()

			fmt.Println("Server adress is : "+getOutboundIP().String())


			go handleConsoleCommands(masterSocketListener)

			for{
				slaveSocketListener , err := masterSocketListener.Accept()
				if err != nil{
					fmt.Println("Error while accepting:",err.Error())
				}

				atomic.AddInt64(&amountOfConnectionsToServer,1)
				
				//clients = append(clients,slaveSocketListener)
				clientName := slaveSocketListener.RemoteAddr().String()
				clients[clientName] = Client{ Conn : slaveSocketListener , Name : clientName }

				go handleRequast(slaveSocketListener)
			}
		case "client":
			consoleReader := bufio.NewReader(os.Stdin)

			fmt.Println("add ip-adress of server")

			serverAddres,err := consoleReader.ReadString('\n')
			if err != nil{
				fmt.Println("Reading ip adress error:",err.Error())
				os.Exit(1)
			}

			serverAddres = subStringBefore(serverAddres,"\n")

			clientSocket , err := net.Dial(CONN_TYPE , net.JoinHostPort(serverAddres,*portPtr))
			if err != nil{
				fmt.Println("Client Connection Error:",err.Error())
				os.Exit(1)
			}
			defer clientSocket.Close()

			scanner := bufio.NewScanner(clientSocket)
			for scanner.Scan(){
				answer := scanner.Text()

				if answer == BREAK{
					fmt.Println("Server disconected")
					break
				}
				fmt.Println("server:" + answer)

				requast,err := consoleReader.ReadString('\n')
				if err != nil{
					fmt.Println("console reading error:",err.Error())
					os.Exit(1)
				}

				clientSocket.Write([]byte(requast))
			}
		case "betterClient":
			consoleReader := bufio.NewReader(os.Stdin)

			fmt.Println("add ip-adress of server")

			serverAddres,err := consoleReader.ReadString('\n')
			if err != nil{
				fmt.Println("Reading ip adress error:",err.Error())
				os.Exit(1)
			}

			serverAddres = subStringBefore(serverAddres,"\n")

			clientSocket , err := net.Dial(CONN_TYPE , net.JoinHostPort(serverAddres,*portPtr))
			if err != nil{
				fmt.Println("Client Connection Error:",err.Error())
				os.Exit(1)
			}
			defer clientSocket.Close()

			fromConsoleCh := make(chan string)
			fromServerCh := make(chan string)

			go readFromConsole(fromConsoleCh)
			go readFromSocket( clientSocket , fromServerCh)

			for{
				var answer string
				select{
				case requast := <- fromConsoleCh:
					clientSocket.Write([]byte(requast))
				case answer = <- fromServerCh:
					if answer == BREAK{
						fmt.Println("Server disconected")
						break
					}
					if strings.HasPrefix(answer,"|"){
						fmt.Println(answer)
				}else{
					fmt.Println("server:" + answer)
			}
		}
				if answer == BREAK{
					break
				}
			}

		default:
			fmt.Println("wrong workflow")
			os.Exit(1)

	}
}

func handleRequast(slaveSocket net.Conn){
	defer socketCloseWithCheck(slaveSocket)

	log.Printf("%s connected",slaveSocket.RemoteAddr().String())
	 
	addr := slaveSocket.RemoteAddr().String()

	slaveSocket.Write([]byte("your remote adress is "+addr+"\n"))

	scanner := bufio.NewScanner(slaveSocket)

	for scanner.Scan(){
		requast := scanner.Text()
		log.Println("from@" + addr + ":"+requast)

		command := subStringBefore(requast,BREAKCHARACTER)
		switch command{
		case GETREQUAST:
			value := subStringAfter(requast,BREAKCHARACTER)
			answer := handleGetRequast(value,slaveSocket)
			writeToSoketWithLog(slaveSocket,answer)
		case POSTREQUAST:
			value := subStringAfter(requast,BREAKCHARACTER)
			answer := handlePostRequast(value,slaveSocket)
			writeToSoketWithLog(slaveSocket,answer)
		case BREAK:
			breakConnection(slaveSocket)
			deleteFromClientsList(slaveSocket)
			break
		case COMMANDERROR:
			slaveSocket.Write([]byte("Add "+ BREAKCHARACTER + " after commmand\n"))
			log.Println("to@" + addr + ":" + "Add "+ BREAKCHARACTER + " after commmand")
		default:
			slaveSocket.Write([]byte("there is no such command\n"))
			log.Println("to@" + addr + ":" + "there is no such command") 
		}
	}

	atomic.AddInt64(&amountOfConnectionsToServer,-1)
}

func handleGetRequast(val string, client net.Conn) string{
	value := subStringBefore(val,BREAKCHARACTER)
	//log.Println("Get value is : "+value)
	switch value{
	case AMOUNTOFCONN:
		return strconv.Itoa(int(amountOfConnectionsToServer)) + " active connections on server side\n"
	case NAMEREQUAST:
		log.Println("name returning start")
		return "Your name is " + clients[client.RemoteAddr().String()].Name+"\n"
	case ONLINE:
		onlineUsersList := "Right now online is ["
		for _,c := range clients{
			onlineUsersList += c.Name + ";"
		}
		onlineUsersList += "]\n"
		return onlineUsersList
	default:
		return "Unknown value for Get requast\n"
	}
}

func handlePostRequast(val string, client net.Conn)string{
	value := subStringBefore(val,BREAKCHARACTER)
	switch value{
	case NAMEREQUAST:
		value = subStringAfter(val,BREAKCHARACTER)
		name := subStringBefore(value,BREAKCHARACTER)
		if name == ""{
			return "Unknown value for Post requast\n"
		}
		if strings.Contains(name," "){
			return "Name cann't contain space\n"
		}
		setNewClientName(name , client)
		return "Your new name is " + name + "\n"
	case MESSAGE:
		userListString := subStringBefore(subStringAfter(val,BREAKCHARACTER),MESSAGEBREAKCHAR)
		userList := strings.Fields(userListString)
		message := "|"+clients[client.RemoteAddr().String()].Name + ":" + subStringAfter(val,MESSAGEBREAKCHAR)+"\n"
		return sendMessageToSliceOfUsers(message , userList)
	default:
		return "Unknown value for Post requast\n"
	}
}

func breakConnection(slaveSocket net.Conn){
	addr := slaveSocket.RemoteAddr().String()
	
	slaveSocket.Write([]byte(BREAK+"\n"))
	atomic.AddInt64(&amountOfConnectionsToServer,-1)
	log.Println("to@" + addr + " : " + BREAK)
	log.Println(addr + "  disconnected")
}

func handleConsoleCommands(serverSocket net.Listener){
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan(){
		command := scanner.Text()
		switch command{
		case "-q":
			for _,connetction := range clients{
				breakConnection(connetction.Conn)
				connetction.Conn.Close()
			}
			serverSocket.Close()
			log.Println("server stoped by console command")
			os.Exit(0)
		
		default:
			fmt.Println("unknown server command")
		}
	}
}

func subStringBefore(str string, character string)string{
	pos := strings.Index(str,character)
	if pos == -1{
		return ""
	}
	return str[0:pos]
}
func subStringAfter(str string,character string)string{
	pos := strings.Index(str,character)
	if pos == -1{
		return ""
	}
	adjPos := pos + len(character)
	if adjPos >= len(str){
		return ""
	}
	return str[adjPos:len(str)]
}

func subStringBetween(str string, character string)string{
	return subStringAfter(subStringBefore(str,character),character)
}

func getOutboundIP() net.IP {
    conn, err := net.Dial("udp", "8.8.8.8:80")
    if err != nil {
        log.Fatal(err)
    }
    defer conn.Close()

    localAddr := conn.LocalAddr().(*net.UDPAddr)

    return localAddr.IP
}

func deleteFromClientsList(client net.Conn)bool{
	delete(clients,client.RemoteAddr().String())
	/*for i,c := range clients{
		if client.RemoteAddr().String() == c.RemoteAddr().String(){
			//clients = append(clients[:i],clients[i+1:]...)
			delete(clients,i)
			return true
		}
	}*/
	return true
}

func socketCloseWithCheck(soket net.Conn){
	if err := soket.Close();err != nil{
		log.Println("Error while closing slave socket:"+err.Error())
	}
}

func setNewClientName(name string, client net.Conn){
	clients[client.RemoteAddr().String()] = Client{ Conn : clients[client.RemoteAddr().String()].Conn , Name : name}
}

func writeToSoketWithLog(socket net.Conn, answer string){
	n,err := socket.Write([]byte(answer))
	if err != nil{
		log.Println("error while writing to soket:" + err.Error())
		str := strconv.Itoa(n)
		log.Println(str + " bytes writed")
	}
	log.Println("to@" + socket.RemoteAddr().String() + ":" + answer)
}

func readFromConsole( ch chan string){
	consoleReader := bufio.NewReader(os.Stdin)
    for{
	requast,err := consoleReader.ReadString('\n')
	if err != nil{
		fmt.Println("console reading error:",err.Error())
		os.Exit(1)
	}

	ch <- requast
	}
}

func readFromSocket(soket net.Conn , ch chan string){
	scanner := bufio.NewScanner(soket)
	for scanner.Scan(){
		answer := scanner.Text()
		ch <- answer
	}
}

func sendMessageToSliceOfUsers(msg string, users []string)string{
	result := "message sended to ["
	for _,reciver := range users{
		for _,client := range clients{
			if client.Name == reciver{
				writeToSoketWithLog(client.Conn,msg)
				result += client.Name + ";"
			}
		}
	}
	return result + "]\n" 
}