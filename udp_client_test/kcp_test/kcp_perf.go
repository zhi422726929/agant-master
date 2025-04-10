package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/xtaci/kcp-go"
)

var (
	// 命令行参数
	host       = flag.String("h", "1.94.191.58", "服务器域名/IP")
	port       = flag.Int("port", 8009, "服务器端口")
	clients    = flag.Int("c", 10, "客户端数量")
	messages   = flag.Int("m", 100, "每客户端发送的消息数")
	packetSize = flag.Int("s", 8, "数据包大小(字节), 包括头部在内的总大小")
	interval   = flag.Duration("i", 10*time.Millisecond, "发送间隔")
	packetLossRate = flag.Float64("p", 0.0, "模拟丢包率 (0.0-1.0)")

	// 并发控制
	wg        sync.WaitGroup
	stateLock sync.Mutex

	// 统计数据
	startTime    time.Time
	connected    int32
	sent         int64
	received     int64
	errors       int32
	latencies    []time.Duration
	totalSent    int32  // 总发送消息数
	totalRecv    int32  // 总接收消息数
	packetsLost  int32  // 丢失的包数量(模拟丢包)
	requestsWithoutResponse int32 // 发出但没收到响应的请求数
	latenciesLock sync.Mutex
	
	// 请求跟踪
	requestTracker = make(map[int32]bool) // 跟踪每个请求是否收到响应
	trackerLock    sync.Mutex
)

func main() {
	// 解析命令行参数
	flag.Parse()

	// 验证丢包率参数
	if *packetLossRate < 0 || *packetLossRate > 1.0 {
		log.Fatalf("丢包率必须在0.0-1.0之间")
	}

	fmt.Println("=== KCP 协议性能测试 ===")
	fmt.Printf("服务器: %s:%d\n", *host, *port)
	fmt.Printf("客户端数: %d\n", *clients)
	fmt.Printf("每客户端消息数: %d\n", *messages)
	fmt.Printf("总计发送: %d 消息\n", *clients**messages)
	fmt.Printf("发送间隔: %v\n", *interval)
	fmt.Printf("模拟丢包率: %.2f%%\n", *packetLossRate*100)
	fmt.Println("========================")

	// 记录开始时间
	startTime = time.Now()

	// 启动客户端
	for i := 0; i < *clients; i++ {
		wg.Add(1)
		go runClient(i)
		// 错开启动时间，避免突发连接
		time.Sleep(5 * time.Millisecond)
	}

	// 等待所有客户端完成
	wg.Wait()

	// 计算测试时长
	duration := time.Since(startTime)

	// 计算请求响应统计
	trackerLock.Lock()
	successfulRequests := 0
	failedRequests := 0
	for _, success := range requestTracker {
		if success {
			successfulRequests++
		} else {
			failedRequests++
		}
	}
	requestsWithoutResponse = int32(failedRequests)
	responseCompletionRate := 0.0
	if len(requestTracker) > 0 {
		responseCompletionRate = float64(successfulRequests) / float64(len(requestTracker)) * 100
	}
	trackerLock.Unlock()

	// 格式化数据量显示
	formatDataSize := func(bytes int64) string {
		if bytes < 1024 {
			return fmt.Sprintf("%d 字节", bytes)
		} else if bytes < 1024*1024 {
			return fmt.Sprintf("%.2f KB", float64(bytes)/1024)
		} else {
			return fmt.Sprintf("%.2f MB", float64(bytes)/(1024*1024))
		}
	}
	
	// 计算吞吐量
	throughput := ""
	if duration.Seconds() > 0 {
		bytesPerSec := float64(sent+received) / duration.Seconds()
		if bytesPerSec < 1024 {
			throughput = fmt.Sprintf("%.2f 字节/秒", bytesPerSec)
		} else if bytesPerSec < 1024*1024 {
			throughput = fmt.Sprintf("%.2f KB/秒", bytesPerSec/1024)
		} else {
			throughput = fmt.Sprintf("%.2f MB/秒", bytesPerSec/(1024*1024))
		}
	}
	
	// 输出测试结果
	fmt.Println("\n=== 测试结果 ===")
	fmt.Printf("测试时长: %v\n", duration)
	fmt.Printf("客户端数量: %d (成功连接: %d)\n", *clients, connected)
	fmt.Printf("每客户端消息数: %d\n", *messages)
	fmt.Printf("计划发送请求总数: %d\n", *clients * *messages)
	fmt.Printf("总发送数据: %s\n", formatDataSize(sent))
	fmt.Printf("总接收数据: %s\n", formatDataSize(received))
	fmt.Printf("错误数: %d\n", errors)
	fmt.Printf("吞吐量: %s\n", throughput)
	fmt.Printf("消息速率: %.2f 消息/秒\n", float64(*clients**messages)/duration.Seconds())
	fmt.Printf("模拟丢包率: %.2f%%\n", *packetLossRate*100)
	fmt.Printf("实际丢包率: %.2f%%\n", float64(packetsLost)/float64(totalSent)*100)
	fmt.Printf("请求响应完成率: %.2f%%\n", responseCompletionRate)
	fmt.Printf("成功收到响应请求: %d\n", successfulRequests)
	fmt.Printf("未收到响应请求: %d\n", failedRequests)
	fmt.Printf("实际发送请求总数: %d\n", totalSent - packetsLost)
	fmt.Printf("实际接收响应总数: %d\n", totalRecv)

	// 计算延迟统计
	if len(latencies) > 0 {
		var totalLatency time.Duration
		maxLatency := latencies[0]
		minLatency := latencies[0]

		for _, l := range latencies {
			totalLatency += l
			if l > maxLatency {
				maxLatency = l
			}
			if l < minLatency {
				minLatency = l
			}
		}

		avgLatency := totalLatency / time.Duration(len(latencies))
		fmt.Printf("平均延迟: %v\n", avgLatency)
		fmt.Printf("最小延迟: %v\n", minLatency)
		fmt.Printf("最大延迟: %v\n", maxLatency)
	} else {
		fmt.Println("没有收集到延迟数据")
	}

	fmt.Println("========================")
}

func runClient(clientID int) {
	defer wg.Done()

	// 连接服务器
	kcpConn, err := kcp.DialWithOptions(fmt.Sprintf("%s:%d", *host, *port), nil, 0, 0)
	if err != nil {
		log.Printf("客户端 %d 连接失败: %v\n", clientID, err)
		stateLock.Lock()
		errors++
		stateLock.Unlock()
		return
	}

	// 设置KCP连接参数
	kcpConn.SetStreamMode(true)
	kcpConn.SetNoDelay(1, 20, 1, 1)
	kcpConn.SetWindowSize(32, 32)
	kcpConn.SetMtu(1280)
	kcpConn.SetReadBuffer(4194304)
	kcpConn.SetWriteBuffer(4194304)
	kcpConn.SetACKNoDelay(true)

	// 短暂等待确保连接稳定
	time.Sleep(100 * time.Millisecond)

	// 设置读写超时
	kcpConn.SetReadDeadline(time.Now().Add(10 * time.Second))
	kcpConn.SetWriteDeadline(time.Now().Add(10 * time.Second))

	// 关闭连接
	defer kcpConn.Close()

	// 更新连接计数
	stateLock.Lock()
	connected++
	stateLock.Unlock()

	// 发送消息
	for i := 0; i < *messages; i++ {
		// 构建心跳包，使用消息ID作为唯一标识用于时间测量
		messageId := rand.Int31()
		msgData := createHeartbeatPacket(messageId)
		
		// 第一个消息，添加额外发送确保连接稳定
		if i == 0 {
			// 先发送一次建立连接，不计入性能测试
			_, err = kcpConn.Write(msgData)
			if err != nil {
				log.Printf("客户端 %d 预热发送失败: %v\n", clientID, err)
				stateLock.Lock()
				errors++
				stateLock.Unlock()
				return
			}
			time.Sleep(500 * time.Millisecond)
		}
		
		// 记录发送时间（在所有准备工作之后）
		sendTime := time.Now()
		
		// 模拟丢包
		if rand.Float64() < *packetLossRate {
			packetsLost++
			totalSent++
			continue
		}
		
		// 发送数据
		n, err := kcpConn.Write(msgData)
		if err != nil {
			log.Printf("客户端 %d 发送失败: %v\n", clientID, err)
			stateLock.Lock()
			errors++
			stateLock.Unlock()
			return
		}

		// 更新发送统计
		stateLock.Lock()
		sent += int64(n)
		totalSent++
		stateLock.Unlock()

		// 接收响应前强制短暂延迟，确保请求已经发送到网络
		time.Sleep(1 * time.Millisecond)
		
		// 接收响应
		response := make([]byte, 1024)
		kcpConn.SetReadDeadline(time.Now().Add(5 * time.Second))
		n, err = kcpConn.Read(response)
		// 计算延迟必须在读取响应后立即进行
		latency := time.Since(sendTime)
		
		if err != nil {
			log.Printf("客户端 %d 接收响应失败: %v\n", clientID, err)
			stateLock.Lock()
			errors++
			stateLock.Unlock()
			trackerLock.Lock()
			requestTracker[messageId] = false
			trackerLock.Unlock()
			continue
		}

		// 将延迟记录添加到全局统计中
		if latency > 0 {  // 确保延迟大于0
			latenciesLock.Lock()
			latencies = append(latencies, latency)
			latenciesLock.Unlock()
		}

		// 更新接收统计
		stateLock.Lock()
		received += int64(n)
		totalRecv++
		stateLock.Unlock()

		// 验证响应
		if n >= 4 {
			respSize := binary.BigEndian.Uint16(response[0:2])
			// 调试前几个客户端和消息的响应
			if clientID < 2 && i == 0 {
				log.Printf("客户端 %d 收到响应: %d字节, 头部大小=%d\n", 
					clientID, n, respSize)
			}
			
			if int(respSize)+2 == n {
				// 检查协议号 (前2字节)
				if len(response) >= 4 && response[2] == 0 && response[3] == 2 {
					// 成功接收心跳应答
					if clientID == 0 && i == 0 {
						log.Printf("客户端 %d 收到正确的心跳应答\n", clientID)
					}
					trackerLock.Lock()
					requestTracker[messageId] = true
					trackerLock.Unlock()
				} else if clientID == 0 && i == 0 {
					log.Printf("客户端 %d 响应协议号异常: %v\n", clientID, response[2:4])
					trackerLock.Lock()
					requestTracker[messageId] = false
					trackerLock.Unlock()
				}
			} else if clientID == 0 && i == 0 {
				log.Printf("客户端 %d 响应大小异常: 期望=%d, 实际=%d\n", 
					clientID, respSize+2, n)
				trackerLock.Lock()
				requestTracker[messageId] = false
				trackerLock.Unlock()
			}
		} else if clientID == 0 && i == 0 {
			log.Printf("客户端 %d 响应太短: %d字节\n", clientID, n)
			trackerLock.Lock()
			requestTracker[messageId] = false
			trackerLock.Unlock()
		}

		// 按照指定间隔发送
		if *interval > 0 {
			time.Sleep(*interval)
		}
	}
}

// 创建心跳请求数据包
func createHeartbeatPacket(messageId int32) []byte {
	// 服务器期望的协议格式: | 2B size | PROTO(2) | AUTO_ID(4) |
	// 确保与已验证的ultra_basic_client.go中的格式完全相同
	
	// 协议号1 = 心跳请求
	protoNum := int16(1)
	
	// 协议头固定部分: 2字节协议号 + 4字节auto_id
	headerSize := 2 + 4
	
	// 计算需要添加的填充数据大小
	// 总大小为 packetSize，需要减去 size字段(2字节) 和 固定头部(6字节)
	paddingSize := *packetSize - 2 - headerSize
	if paddingSize < 0 {
		// 如果指定的包大小小于最小所需大小，使用最小所需大小
		paddingSize = 0
	}
	
	// 数据部分大小: 固定头部 + 填充部分
	dataSize := headerSize + paddingSize
	
	// 创建完整的数据包: 2字节大小字段 + 数据部分
	packet := make([]byte, 2+dataSize)
	
	// 写入数据大小（不包括大小字段本身的2字节）
	binary.BigEndian.PutUint16(packet[0:2], uint16(dataSize))
	
	// 写入协议号 (2字节)
	binary.BigEndian.PutUint16(packet[2:4], uint16(protoNum))
	
	// 写入auto_id (4字节整数)
	binary.BigEndian.PutUint32(packet[4:8], uint32(messageId))
	
	// 如果有填充数据，添加随机填充
	if paddingSize > 0 {
		padding := packet[8:8+paddingSize]
		for i := range padding {
			padding[i] = byte(rand.Intn(256))
		}
	}
	
	return packet
}
