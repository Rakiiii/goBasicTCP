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
)

const(
	CONN_TYPE = "tcp"
	BREAK = "Break"
	GETREQUAST = "Get"
	AMOUNTOFCONN = "Amount Of Connections"
	COMMANDERROR = ""
	BREAKCHARACTER = ":"
)

var amountOfConnectionsToServer int64
var clients = make([]net.Conn,0)

func main(){
	switch{
	case len(os.Args) <= 2:
		fmt.Fprintf(os.Stderr,"Usage: %s workflow,port\n",os.Args[0])
		os.Exit(1)
	case len(os.Args) != 3:
		fmt.Fprintf(os.Stderr,"%s too many arguments",os.Args[0])
		os.Exit(1)
	}

	workflowPtr := flag.String("workflow","crash","a string")
	portPtr := flag.String("port","8080","a string")

	flag.Parse()

	fmt.Println("workflow="+*workflowPtr + " port:"+*portPtr)
	switch *workflowPtr{
		case "server":
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
				
				clients = append(clients,slaveSocketListener)


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
			answer := handleGetRequast(value)
			slaveSocket.Write([]byte(answer))
			log.Println("to@" + addr + ":" + answer)
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

func handleGetRequast(val string) string{
	value := subStringBefore(val,BREAKCHARACTER)
	//log.Println("Get value is : "+value)
	switch value{
	case AMOUNTOFCONN:
		return strconv.Itoa(int(amountOfConnectionsToServer)) + " active connections on server side\n"
	default:
		return "Unknown value for Get requast\n"
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
				breakConnection(connetction)
				connetction.Close()
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
	for i,c := range clients{
		if client.RemoteAddr().String() == c.RemoteAddr().String(){
			clients = append(clients[:i],clients[i+1:]...)
			return true
		}
	}
	return false
}

func socketCloseWithCheck(soket net.Conn){
	if err := soket.Close();err != nil{
		log.Println("Error while closing slave socket:"+err.Error())
	}
}