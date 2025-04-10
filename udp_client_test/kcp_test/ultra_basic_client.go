// 极简KCP客户端，完全按照服务端期望格式构造数据包
package main

import (
	"encoding/binary"
	"flag"
	"log"
	"time"

	"github.com/xtaci/kcp-go"
)

func main() {
	// 命令行参数
	serverAddr := flag.String("server", "1.94.191.58:8009", "服务器地址")
	flag.Parse()

	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)
	log.Printf("开始最基础的KCP测试，连接到 %s\n", *serverAddr)

	// 创建KCP连接
	kcpConn, err := kcp.DialWithOptions(*serverAddr, nil, 0, 0)
	if err != nil {
		log.Fatalf("KCP连接失败: %v\n", err)
		return
	}
	defer kcpConn.Close()

	// 设置与服务端完全相同的KCP参数
	kcpConn.SetStreamMode(true)
	kcpConn.SetNoDelay(1, 20, 1, 1)
	kcpConn.SetWindowSize(32, 32)
	kcpConn.SetMtu(1280)
	kcpConn.SetReadBuffer(4194304)
	kcpConn.SetWriteBuffer(4194304)
	kcpConn.SetACKNoDelay(true)
	
	log.Printf("KCP连接已建立，并设置了与服务端相同的参数\n")
	time.Sleep(1 * time.Second)

	// 构建心跳请求数据包
	// 服务器期望的格式: | SIZE(2) | PROTO(2) | DATA |
	// 心跳请求需要包含一个int16协议号和32位整数auto_id
	protoNum := int16(1) // 协议号1 = 心跳请求
	autoId := int32(123) // 随机的auto_id值

	// 计算数据部分的大小: 2字节协议号 + 4字节auto_id
	dataSize := 2 + 4 
	
	// 创建完整的数据包: 2字节大小字段 + 数据部分
	packet := make([]byte, 2+dataSize)
	
	// 写入数据大小（不包括大小字段本身的2字节）
	binary.BigEndian.PutUint16(packet[0:2], uint16(dataSize))
	
	// 写入协议号 (2字节)
	binary.BigEndian.PutUint16(packet[2:4], uint16(protoNum))
	
	// 写入auto_id (4字节整数)
	binary.BigEndian.PutUint32(packet[4:8], uint32(autoId))

	log.Println("准备发送心跳包, 总长度:", len(packet), "字节")
	log.Println("数据格式: | SIZE(2):", dataSize, "| PROTO(2):", protoNum, "| AUTO_ID(4):", autoId, "|")
	
	// 打印十六进制格式的完整数据包
	log.Printf("数据包(十六进制): ")
	for i, b := range packet {
		log.Printf("%02X ", b)
		if (i+1) % 16 == 0 {
			log.Printf("\n")
		}
	}
	log.Printf("\n")
	
	// 发送数据包
	n, err := kcpConn.Write(packet)
	if err != nil {
		log.Fatalf("发送数据失败: %v\n", err)
		return
	}
	log.Printf("发送成功: %d字节\n", n)
	
	// 再发送一次，确保数据到达
	time.Sleep(1 * time.Second)
	n, err = kcpConn.Write(packet)
	if err != nil {
		log.Fatalf("第二次发送失败: %v\n", err)
		return
	}
	log.Printf("第二次发送成功: %d字节\n", n)
	
	// 尝试接收响应，心跳包的响应应该是协议号2 (heart_beat_ack)
	buffer := make([]byte, 1024)
	kcpConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	
	readN, err := kcpConn.Read(buffer)
	if err != nil {
		log.Printf("读取响应失败: %v\n", err)
	} else {
		log.Printf("接收到响应: %d字节\n", readN)
		
		// 尝试解析响应
		if readN >= 2 {
			// 解析SIZE字段
			responseSize := binary.BigEndian.Uint16(buffer[0:2])
			log.Printf("响应SIZE字段: %d\n", responseSize)
			
			if int(responseSize)+2 <= readN {
				// 读取完整的响应数据
				responseData := buffer[2:2+int(responseSize)]
				
				// 解析协议号
				if len(responseData) >= 2 {
					respProto := binary.BigEndian.Uint16(responseData[0:2])
					
					log.Printf("响应协议号: %d\n", respProto)
					
					// 检查是否是心跳包应答(协议号2)
					if respProto == 2 {
						log.Printf("成功接收到心跳包应答!\n")
					} else {
						log.Printf("收到非心跳包应答的响应，协议号: %d\n", respProto)
					}
					
					// 如果有其他数据
					if len(responseData) > 2 {
						respPayload := responseData[2:]
						log.Printf("附加数据: %v\n", respPayload)
					}
				} else {
					log.Printf("响应数据格式不正确，长度: %d\n", len(responseData))
				}
			} else {
				log.Printf("响应数据不完整: 期望 %d 字节，收到 %d 字节\n", responseSize+2, readN)
			}
		} else {
			log.Printf("响应太短，无法解析\n")
		}
	}
	
	log.Printf("测试完成\n")
}
