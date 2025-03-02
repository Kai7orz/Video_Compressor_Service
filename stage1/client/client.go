package main 


import (
	"net"
	"log"
	"io"
	"fmt"
	"os"
	"encoding/binary"
)



func sendData(data []byte,tcpConn net.Conn) {
//1500bytes送るときの流れを考える
	var writeCounter int = 0
	var n int = 0
	var headerSize int = 8
	currentSize := 0 //送信済みのデータカウント
	leftDataSize := len(data) //ファイルのバイト長計算・残りの送信するべきデータサイズ

	log.Println("client sent header--->",data[0:headerSize])
	_,err := tcpConn.Write(data[0:headerSize]) //最初のヘッダー送信
	writeCounter += 1
	if err != nil{
		log.Println(err)
	}
	leftDataSize -= headerSize
	currentSize += headerSize

	for{
		if leftDataSize <= 1400 {  //最後の送信処理記述
			sendBuffer := make([]byte,leftDataSize)
			copy(sendBuffer[0:leftDataSize],data[currentSize:currentSize+leftDataSize]) 
			n,err = tcpConn.Write(sendBuffer[0:leftDataSize])
			writeCounter += 1
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
		writeCounter += 1
		if err != nil{
			log.Fatal(err,n)
		}
	}

	log.Println("write Counter",writeCounter)

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

	headerBuf := make([]byte,8)  //ヘッダー用のバッファーを作成
	

	jsonData := `{
		"command" : ["ffmpeg"],
		"option":["-i","-ss","-c","-t"],
		"ope":["output1.mp4","00:00:1.0","copy","00:00:5.0"],
		"output":["c_output5.mp4"]
				}`
	fmt.Println("jsondata::",[]byte(jsonData))
	lenBuf := make([]byte,2)
	jsonDataSize :=	uint16(len([]byte(jsonData))) //json のサイズ
	binary.BigEndian.PutUint16(lenBuf[0:2],jsonDataSize) //json のサイズを2バイトで表現
	copy(headerBuf[0:2],lenBuf)

	lenBuf = make([]byte,1)
	mediaType := []byte("mp4") //media の種類
	mediaTypeSize := uint8(len(mediaType))
	lenBuf[0]=mediaTypeSize
	copy(headerBuf[2:3],lenBuf)

	lenBuf = make([]byte,8)
	binary.BigEndian.PutUint64(lenBuf,uint64(fileSize))  // [0 0 0 10 112 54 33 122] となるのでlenBuf[0:5]を利用
	log.Println("payloadSize:",lenBuf)
	copy(headerBuf[3:8],lenBuf[3:8]) 
	headerSize := len(headerBuf)
	//ここまでがヘッダー作成
	// ここからがペイロード作成部分
	//JSONファイル+ メディアタイプ+ filesize分のデータ
	data := make([]byte,len(jsonData)+len(mediaType)+fileSize)
	data = append([]byte(jsonData),mediaType...)
	data = append(data,mpBuffer[0:fileSize]...)

	buf := make([]byte,headerSize+len(data))

    copy(buf[0:headerSize],headerBuf) //パケットのヘッダーを付加	
	fmt.Println("Client 送信ヘッダーサイズ:",headerSize)
	fmt.Println("jsonDataSize:",jsonDataSize," mediatypeSize:",mediaTypeSize," payloadSize:",fileSize)
	
	copy(buf[headerSize:headerSize+len(data)],data) //bodyをパケットに追加	
	/*
	fmt.Println(mpBuffer[0:fileSize])
	fmt.Println("送信動画ファイルサイズ:",fileSize)
	*/
	file.Close()
	return buf
}

func receiveData(conn net.Conn){
	var leftSize uint64 
	var currentSize int = 0

	
	headerBuf := make([]byte,8) 
	n,err := io.ReadFull(conn,headerBuf) //ヘッダーパケットを読み込む
	if err != nil || n!=8{
		log.Fatal(err)
	}

	jsonDataSize := binary.BigEndian.Uint16(headerBuf[0:2]) //or buf[6:8]
	mediaTypeSize := uint8(headerBuf[2])
	
	adjBuf := make([]byte,3)
	list := []byte{0,0,0}
	adjBuf = append(list,headerBuf[3:8]...)
	log.Println("adjBuf:",adjBuf)

	payloadSize := binary.BigEndian.Uint64(adjBuf)
	leftSize = payloadSize //受信するべき動画ファイルのバイト数

	mpBuffer := make([]byte,payloadSize)

	recvBuf := make([]byte,1400)
	n,err = io.ReadFull(conn,recvBuf)  //1400バイト以下のファイルは受け付けない状態になっていることに注意
	if err != nil{
		fmt.Println(err)
	}

	jsonAndTypeSize := int(jsonDataSize+uint16(mediaTypeSize))

	jsonData := recvBuf[0:jsonDataSize] //json の読み込み
	mediaType := recvBuf[jsonDataSize:jsonAndTypeSize] //mediaの種類
	payload := recvBuf[jsonAndTypeSize:n] //動画・画像ファイル本体

	log.Println("jsonData:",jsonData," mediaType:",mediaType)
	log.Println("pay10",payload[0:10])
	copy(mpBuffer[0:n-jsonAndTypeSize],payload)  //動画・画像ファイルはmpbufferへ記録
	currentSize = n-jsonAndTypeSize
	leftSize = leftSize - (uint64(n))+uint64(len(jsonData)+len(mediaType)) //残りの読み込みデータ数

	for{
		if leftSize <= 1400{ 	//最後の受信処理
			buf := make([]byte,leftSize) 
			n,err := io.ReadFull(conn,buf)
			if err != nil{
				log.Println(err)
			}
			log.Println("final process read -->",n," bytes")
			copy(mpBuffer[currentSize:currentSize+n],buf[:n])
			currentSize += n
			leftSize -= uint64(n)

			log.Println("finished read size:",currentSize)
			break
		}

		buf := make([]byte,1400) 
		n,err := io.ReadFull(conn,buf) //1400バイトの読み込みが期待される
		if err != nil || n != 1400{
			log.Println("leftSize:",leftSize)
			log.Fatal(err)
		}
		//fmt.Println(buf)
		copy(mpBuffer[currentSize:currentSize+n],buf[:n])
		currentSize += n
		leftSize -= uint64(n)
	}

//	fmt.Println(mpBuffer[0:currentSize-1])

	outputFile := "client_output.mp4"
	err = os.WriteFile(outputFile,mpBuffer[0:currentSize],0644) //権限0644 --rwとかのやつ
	if err != nil{
		log.Fatal(err)
	}


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
	
	receiveData(conn)



	readBuffer := make([]byte,1024)
	_,err  :=  conn.Read(readBuffer)
	if err != nil{
		log.Fatal(err)
	}
	//fmt.Println("Receive message from server: ",readBuffer)

}