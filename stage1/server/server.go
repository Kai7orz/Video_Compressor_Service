package main 

import (

	"net"
	"log"
	"encoding/binary"
	"os"
	"fmt"
	"strconv"
	"os/exec"
	"encoding/json"
	"reflect"
)

type MyCommand struct{
	Command []string 
	Option []string 
	Ope []string 
	Output []string 
}


func doFfmpeg(c MyCommand){
	var cmdArray []string
	cmdArray = append(cmdArray,c.Command[0])
	for i:=0;i<len(c.Option);i++{
		cmdArray=append(cmdArray,c.Option[i])
		cmdArray=append(cmdArray,c.Ope[i])
	}
	cmdArray = append(cmdArray,c.Output[0])
	fmt.Println("cmdArray:",cmdArray)
	_, err := exec.Command(cmdArray[0], cmdArray[1:]...).Output()
	if err != nil {
		log.Println(err)
	}
}

func sendComplete(conn net.Conn) {
	defer conn.Close()
	message := "connection finished successfully"
	conn.Write([]byte(message))
}


func serverRead(conn net.Conn,id int){
	
	var leftSize uint32	//残り読み込むべきデータサイズしたがってこの値でファイルの読み込みデータ数を管理するを管理する
	const GB = 1024 * 1024 * 1024
	currentSize := 0	//現在まで読み込んだデータサイズ

	buf := make([]byte,32) //1回の通信で受信したデータ
	n,_ := conn.Read(buf) //ヘッダーパケットを読み込む

	fmt.Println("READ HEADER SIZE ",n) //32bytes が期待される
	
	leftSize = binary.BigEndian.Uint32(buf[28:32]) //バイト列から整数に変換(データ長読み込み)	
	tmp := leftSize //受信するべき全バイト数
	mpBuffer := make([]byte,tmp)

	log.Println("first leftSize:",leftSize)
	if leftSize >= 1400{
	for{

		if leftSize <= 1400{ 	//最後の受信処理
			buf := make([]byte,leftSize) 
			n,err := conn.Read(buf)
			if err != nil{
				log.Println("leftSize <= 1400")
				log.Println(err)
			}
			log.Println("final process read -->",n," bytes")
			copy(mpBuffer[currentSize:],buf[:n])
			break
		}

		buf := make([]byte,1400) 
		n,err := conn.Read(buf) //1400バイトの読み込みが期待される
		if err != nil{
			log.Println("normal loop")
			log.Fatal(err)
		}

		fmt.Println("全データ数:",tmp)
		log.Println("receive data from client ",n," bytes")

		copy(mpBuffer[currentSize:currentSize+n],buf[:n])
		currentSize += n
		leftSize -= uint32(n)
		fmt.Println("leftsize:",leftSize)
	}
	} else{ //1回しかmp4データ送信しなくてよいときの処理
		buf := make([]byte,1400) 
		n,err := conn.Read(buf) //1400バイトの読み込みが期待される
		if err != nil{
			log.Fatal(err)
		}
	copy(mpBuffer[currentSize:currentSize+n],buf[:n])
	currentSize += n
}

	outputFile := "output"+strconv.Itoa(id)+".mp4"
	err := os.WriteFile(outputFile,mpBuffer[0:currentSize],0644) //権限0644 --rwとかのやつ
	if err != nil{
		log.Fatal(err)
	}

	jsonData := `{
		"command" : ["ffmpeg"],
		"option":["-i","-ss","-c","-t"],
		"ope":["output1.mp4","00:00:1.0","copy","00:00:5.0"],
		"output":["c_output4.mp4"]
	}`
	fmt.Println(reflect.TypeOf(jsonData))
	

	var myCom MyCommand 
	err = json.Unmarshal([]byte(jsonData),&myCom)
	if err != nil{
		fmt.Println(err)
	}
	
	doFfmpeg(myCom) //ffmpegの実行

	sendComplete(conn) //送信完了処理(コネクションの切断)


}

func serverStart() {
	id := 0
	tcpAddress,err := net.ResolveTCPAddr("tcp","localhost:8080")
	if err != nil{
		log.Fatal(err)
	}
	
	
	ln,err := net.ListenTCP("tcp",tcpAddress)
	if err != nil{
		log.Fatal(err)
	}
for{
	conn,err := ln.Accept()
	id += 1
	if err != nil{
		log.Fatal(err)
	}
	defer conn.Close()
		go serverRead(conn,id)
	}
}
