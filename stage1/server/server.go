package main 

import (

	"net"
	"log"
	"encoding/binary"
	"fmt"
	"os/exec"
	"encoding/json"
	"io"
	"os"
	"strconv"
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
	output, err := exec.Command(cmdArray[0], cmdArray[1:]...).CombinedOutput()
	if err != nil {
		log.Println(output)
		log.Println(err)
	}

	
}
func serverSend(data []byte,tcpConn net.Conn) {
	//1500bytes送るときの流れを考える


		var dividedSize int = 1400
		var n int = 0
		var headerSize int = 8
		var currentSize int = 0 //送信済みのデータカウント
		var leftDataSize int= len(data) //ファイルのバイト長計算・残りの送信するべきデータサイズ
	
		_,err := tcpConn.Write(data[0:headerSize]) //最初のヘッダー送信
		if err != nil{
			log.Println(err)
		}

	
		leftDataSize -= headerSize
		currentSize += headerSize
	
		for{
			if leftDataSize <= dividedSize {  //最後の送信処理記述
				sendBuffer := make([]byte,leftDataSize)
				copy(sendBuffer[0:leftDataSize],data[currentSize:currentSize+leftDataSize]) 

				n,err = tcpConn.Write(sendBuffer[0:leftDataSize])
				log.Println("Data send to client",len(sendBuffer[0:leftDataSize])," Byte")
				if err != nil{
					log.Fatal(err,n)
				}
				break
			}
	
			sendBuffer := make([]byte,dividedSize)

			copy(sendBuffer[0:dividedSize],data[currentSize:currentSize+1400])
			currentSize += dividedSize
			leftDataSize -= dividedSize
	
//			fmt.Println(sendBuffer)
			n,err = tcpConn.Write(sendBuffer)
			if err != nil{
				log.Fatal(err,n)
			}
		}
	}
	

func makeResponse(c MyCommand) []byte{

	file,err := os.Open(c.Output[0]) //ファイルのオープン
	if err != nil{
		log.Fatal(err)
	}


	const GB = 1024 * 1024 * 1024
	genBuffer := make([]byte, 4*GB) //処理後のファイル読み込みのためのバッファ
	fileSize,err := file.Read(genBuffer) //ファイルの読み込み
	
	if err != nil{
		log.Println(err)
	}

	headerBuf := make([]byte,8) 

	jsonBuf := make([]byte,2)
	binary.BigEndian.PutUint16(jsonBuf,uint16(1)) //長さ1
	copy(headerBuf[0:2],jsonBuf)

	mediaType := []byte("mp4")
	mediaTypeSize := len(mediaType)
	headerBuf[2] = byte(mediaTypeSize)

	payloadBuf := make([]byte,8)
	binary.BigEndian.PutUint64(payloadBuf,uint64(fileSize))
	copy(headerBuf[3:8],payloadBuf[3:8])
	headerSize := len(headerBuf)
	log.Println("動画のファイルサイズ:",fileSize)
	log.Println("server header:",headerBuf)

	//以下がjson+mediatype+処理後のファイル 	jsonはデータなし0をつけとく
	responseData := make([]byte,1+mediaTypeSize+fileSize) //json は0の1バイトのみ確保で十分
	jb := make([]byte,1)
	copy(jb[0:1],[]byte{0})
	copy(responseData[0:1],jb)
	copy(responseData[1:1+mediaTypeSize],mediaType)
	fmt.Println("res",responseData[0:10])
	copy(responseData[1+mediaTypeSize:1+mediaTypeSize+fileSize],genBuffer[0:fileSize])

	
	responseBuffer := make([]byte,headerSize + len(responseData))
	copy(responseBuffer[0:headerSize],headerBuf)
	copy(responseBuffer[headerSize:headerSize+len(responseData)],responseData)

/*	log.Println("Server send data :",responseBuffer)
	log.Println("responsebuffer size:",len(responseBuffer))
	log.Println("response ",responseData[0:10])
*/
	file.Close()
	return responseBuffer
}

func sendComplete(c MyCommand,conn net.Conn) {
	message := "connection finished successfully"
	conn.Write([]byte(message))

	for i:=0;i<len(c.Option);i++{
		if c.Option[i] == "-i"{
			err :=os.Remove(c.Ope[i])
			if err != nil{
				log.Println(err)
			}
		}
	}
	err := os.Remove(c.Output[0])
	if err != nil{
		log.Println(err)
	}
}


func serverRead(conn net.Conn,id int){
	var readCounter int = 0
	var leftSize uint64	//残り読み込むべきデータサイズしたがってこの値でファイルの読み込みデータ数を管理するを管理する
	const GB = 1024 * 1024 * 1024
	currentSize := 0	//mpBufferの管理インデックス(データの読み込み数)

	buf := make([]byte,8) 
	n,err := io.ReadFull(conn,buf) //ヘッダーパケットを読み込む
	if err != nil || n!=8{
		log.Fatal(err)
	}
	
	readCounter += 1

	fmt.Println("READ HEADER SIZE ",n) //8bytes が期待される
	
	jsonDataSize := binary.BigEndian.Uint16(buf[0:2]) //or buf[6:8]
	mediaTypeSize := uint8(buf[2])
	
	adjBuf := make([]byte,3)
	list := []byte{0,0,0}
	adjBuf = append(list,buf[3:8]...)
	log.Println("adjBuf:",adjBuf)
	payloadSize := binary.BigEndian.Uint64(adjBuf)
	//log.Println("jsonデータサイズ:",jsonDataSize," メディアタイプサイズ:",mediaTypeSize," ペイロードサイズ:",payloadSize)
	leftSize = payloadSize //受信するべき動画ファイルのバイト数
	mpBuffer := make([]byte,payloadSize)

	//最初のパケットはjson + mediatype + payload ,それ以降はpayloadのみなので，最初のパケットだけ例外的な処理を行う
	recvBuf := make([]byte,1400)
	n,err = io.ReadFull(conn,recvBuf)  //1400バイト以下のファイルは受け付けない状態になっていることに注意
	readCounter += 1
	if err != nil{
		fmt.Println(err)
	}

	jsonAndTypeSize := int(jsonDataSize+uint16(mediaTypeSize))

	jsonData := recvBuf[0:jsonDataSize] //json の読み込み
	mediaType := recvBuf[jsonDataSize:jsonAndTypeSize] //mediaの種類
	payload := recvBuf[jsonAndTypeSize:n] //動画・画像ファイル本体
/*
	log.Println("header Loading...")
	log.Println("header jsonData ",jsonData)
	log.Println(" mediaType:",mediaType)
	log.Println("n-jsontypesize",n-jsonAndTypeSize)
*/
	copy(mpBuffer[0:n-jsonAndTypeSize],payload)  //動画・画像ファイルはmpbufferへ記録
	currentSize = n-jsonAndTypeSize
	leftSize = leftSize - (uint64(n))+uint64(len(jsonData)+len(mediaType)) //残りの読み込みデータ数
	//leftsizeが動画ファイルのみの残りのロード数用の変数だったので本来は上記式
	//が正しいのに，leftSize -= uint64(n)　として，余分にjson,mediatypeのサイズまで引いてしまったので残りの動画ファイル読み込みサイズがずれた
	//これが原因で末尾だけおかしなデータになっていた
	for{

		if leftSize <= 1400{ 	//最後の受信処理
			buf := make([]byte,leftSize) 
			n,err := io.ReadFull(conn,buf)
			readCounter += 1
			if err != nil{
				log.Println(err)
			}
			log.Println("final process read -->",n," bytes")
			copy(mpBuffer[currentSize:currentSize+n],buf[:n])
			currentSize += n
			leftSize -= uint64(n)

			//fmt.Println(mpBuffer[:payloadSize])

			log.Println("finished read size:",currentSize)
			break
		}

		buf := make([]byte,1400) 
		n,err := io.ReadFull(conn,buf) //1400バイトの読み込みが期待される
		readCounter += 1
		if err != nil || n != 1400{
			log.Println("leftSize:",leftSize)
			log.Fatal(err)
		}

		//fmt.Println("全データ数:",payloadSize)
		//log.Println("receive data from client ",n," bytes")
		copy(mpBuffer[currentSize:currentSize+n],buf[:n])
		currentSize += n
		leftSize -= uint64(n)
//		fmt.Println("leftsize:",leftSize)
}
/*
	fmt.Println("動画・画像データサイズ:",currentSize)
	fmt.Println("jsonDataSize:",len(jsonData)," mediaTypeSize:",mediaTypeSize," payload:",payloadSize)
	fmt.Println("server read Counter",readCounter)
*/

	outputFile := "output"+strconv.Itoa(id)+".mp4"
	err = os.WriteFile(outputFile,mpBuffer[0:currentSize],0644) //権限0644 --rwとかのやつ
	if err != nil{
		log.Fatal(err)
	}

	var myCom MyCommand 
	err = json.Unmarshal(jsonData,&myCom)
	if err != nil{
		fmt.Println(err)
	}
	
	doFfmpeg(myCom) //ffmpegの実行
	fmt.Println("-----------------Server send----------------------")
	serverSend(makeResponse(myCom),conn)
	sendComplete(myCom,conn) //送信完了処理(コネクションの切断)	
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
