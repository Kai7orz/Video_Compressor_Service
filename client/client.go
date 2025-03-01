package main 


import (
	"net"
	"log"
	"fmt"
	"os"
	"encoding/binary"
)



func sendData(data []byte,tcpConn net.Conn) {
//1500bytes送るときの流れを考える
	var n int = 0
	var err error 
	currentSize := 0 //送信済みのデータカウント
	leftDataSize := len(data) //ファイルのバイト長計算・残りの送信するべきデータサイズ
	//最初は32バイトのヘッダーを送信する


	tcpConn.Write(data[0:32]) //最初のヘッダー送信
	leftDataSize -= 32
	currentSize += 32	

	for{
		if leftDataSize <= 1400 {  //最後の送信処理記述
			sendBuffer := make([]byte,1400)
			copy(sendBuffer[0:leftDataSize],data[currentSize:currentSize+leftDataSize]) 
			n,err = tcpConn.Write(sendBuffer[0:leftDataSize])
			log.Println("Data send to server ",len(sendBuffer[0:leftDataSize])," Byte")
			if err != nil{
				log.Fatal(err,n)
			}
			break
		}

		sendBuffer := make([]byte,1400)
		copy(sendBuffer[0:1400],data[currentSize:currentSize+1400])
		currentSize += 1400
		leftDataSize -= 1400


		n,err = tcpConn.Write(sendBuffer)
		if err != nil{
			log.Fatal(err,n)
		}
	}

}


func makeData(filePath string) []byte{	//送信するべき全てのデータ(バイト列)を返す
	file,err := os.Open(filePath)
	if err != nil{
		fmt.Println("Error opening file:",err)
		return []byte{}
	}


	const GB = 1024 * 1024 * 1024
	mpBuffer := make([]byte, 4*GB) //
	fileSize,err := file.Read(mpBuffer) //ここでファイルサイズを取得し，ヘッダーにその情報を載せる
	if err != nil{
		log.Fatal(err)
	}

	headerBuf := make([]byte,32)  //ヘッダー用のバッファーを作成
	lenBuf := make([]byte,4)
	binary.BigEndian.PutUint32(lenBuf, uint32(fileSize))
	 //データをバイト列に変換
	copy(headerBuf[32-len(lenBuf):],lenBuf) //ビッグエンディアンで32バイト列に変換
	copy(mpBuffer[32:32+fileSize],mpBuffer[0:fileSize]) //送信するべきすべてのデータbufferの作成(mp4ファイル本体のデータを付加)
	copy(mpBuffer[0:32],headerBuf) //パケットのヘッダーを付加
	fmt.Println("Client Header Show Size:",fileSize)
	fmt.Println("Client 送信データサイズ:",len(mpBuffer[0:32+fileSize]))
	file.Close()


	return mpBuffer[0:32+fileSize]
}

func clientStart(serverAddress string){
	//サーバへ接続を行うか選択
	for{
		var userPermission string
		fmt.Println("Connect to Servre?? (y or n)")
		fmt.Scan(&userPermission)	

		if userPermission != "y"{
		fmt.Println("selected n or invalid input")
		continue
	}
		break
	}


	conn,_ := net.Dial("tcp",serverAddress)
	defer conn.Close()

	var filePath string 
	fmt.Println("Choose file ")
	fmt.Scan(&filePath)
	data := makeData(filePath)
	sendData(data,conn)
	readBuffer := make([]byte,1024)
	_,err  :=  conn.Read(readBuffer)
	if err != nil{
		log.Fatal(err)
	}
	fmt.Println("Receive message from server: ",string(readBuffer))

}