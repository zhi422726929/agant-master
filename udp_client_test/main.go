package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/xtaci/kcp-go"
)

var (
	server     = flag.String("server", "127.0.0.1:8009", "服务器地址")
	clients    = flag.Int("clients", 1, "并发客户端数量")
	messages   = flag.Int("messages", 100, "每个客户端发送的消息数")
	packetSize = flag.Int("size", 512, "消息大小（字节）")
	interval   = flag.Duration("interval", 10*time.Millisecond, "发送消息间隔")
	showDetail = flag.Bool("detail", false, "是否显示详细日志")
	timeout    = flag.Duration("timeout", 5*time.Second, "连接超时时间")

	totalSent      int64
	totalReceived  int64
	errorCount     int64
	connectedCount int64
	latencies      []time.Duration
	latencyMutex   sync.Mutex
	mu             sync.Mutex
)

func main() {
	// 解析命令行参数
	flag.Parse()

	fmt.Printf("开始测试: 连接到 %s, %d个客户端, 每个客户端发送 %d 条消息，包大小: %d 字节\n",
		*server, *clients, *messages, *packetSize)

	// 初始化统计数据
	latencies = make([]time.Duration, 0, *clients**messages)

	// 创建等待组
	var wg sync.WaitGroup
	wg.Add(*clients)

	// 启动时间
	startTime := time.Now()

	// 启动客户端
	for i := 0; i < *clients; i++ {
		go func(clientID int) {
			defer wg.Done()
			runKCPTest(clientID, *messages, *packetSize, *server, *showDetail)
		}(i)

		// 小延迟，避免一次性创建太多连接
		time.Sleep(5 * time.Millisecond)
	}

	// 打印进度信息
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				connected := atomic.LoadInt64(&connectedCount)
				sent := atomic.LoadInt64(&totalSent)
				errors := atomic.LoadInt64(&errorCount)
				elapsed := time.Since(startTime)

				fmt.Printf("进行中 [%v]: 已连接: %d/%d 客户端, 已发送: %.2f MB, 错误: %d\n",
					elapsed.Round(time.Second),
					connected, *clients,
					float64(sent)/(1024*1024),
					errors)
			}
		}
	}()

	// 等待所有客户端完成或超时
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// 所有客户端完成
	case <-time.After(10 * time.Minute): // 设置一个最大运行时间
		fmt.Println("测试超时，强制结束")
	}

	// 计算总时间
	duration := time.Since(startTime)

	// 计算并显示结果
	showResults(duration)
}

func runKCPTest(clientID int, numMessages, packetSize int, serverAddr string, detail bool) {
	// 解析服务器地址
	serverAddr = *server
	if _, _, err := net.SplitHostPort(serverAddr); err != nil {
		serverAddr = fmt.Sprintf("%s:8009", serverAddr)
	}

	// 记录连接开始时间
	connectStart := time.Now()

	// 创建KCP连接
	block, _ := kcp.NewNoneBlockCrypt(nil) // 不使用加密
	kcpConn, err := kcp.DialWithOptions(serverAddr, block, 10, 3)
	if err != nil {
		log.Printf("[Client %d] KCP连接失败: %v\n", clientID, err)
		mu.Lock()
		errorCount++
		mu.Unlock()
		return
	}
	defer kcpConn.Close()

	// 设置KCP参数 - 与服务端完全匹配
	kcpConn.SetStreamMode(true)
	kcpConn.SetNoDelay(1, 20, 1, 1)
	kcpConn.SetWindowSize(32, 32)
	kcpConn.SetMtu(1280)
	kcpConn.SetReadBuffer(4194304)
	kcpConn.SetWriteBuffer(4194304)
	kcpConn.SetACKNoDelay(true)
	
	// 设置DSCP值
	err = kcpConn.SetDSCP(46)
	if err != nil {
		log.Printf("[Client %d] 设置DSCP失败: %v\n", clientID, err)
	}
	
	// 设置超时时间
	kcpConn.SetDeadline(time.Now().Add(30 * time.Second))

	log.Printf("[Client %d] 连接成功，耗时: %v\n", clientID, time.Since(connectStart))
	log.Printf("[Client %d] KCP连接已建立，参数已设置，确保匹配服务端配置\n", clientID)

	// 等待确保连接稳定
	time.Sleep(300 * time.Millisecond)
	
	// 发送握手消息 - 极度简化，严格按照 | 2B size | DATA | 格式
	log.Printf("[Client %d] 使用严格的消息格式: | 2B size | DATA |\n", clientID)
	
	// 尝试发送几种不同的简单消息
	handshakes := []string{
		"HELLO",
		"HANDSHAKE",
		"CONNECT",
	}
	
	var sentSuccess bool
	
	for i, data := range handshakes {
		// 生成数据
		dataBytes := []byte(data)
		
		// 计算数据长度
		dataSize := len(dataBytes)
		
		// 创建完整消息包: | 2B size | DATA |
		packet := make([]byte, 2+dataSize)
		
		// 设置2字节的SIZE - 表示后续DATA部分的长度
		binary.BigEndian.PutUint16(packet[0:2], uint16(dataSize))
		
		// 设置DATA
		copy(packet[2:], dataBytes)
		
		// 详细日志
		log.Printf("[Client %d] 发送握手消息 #%d: SIZE=%d, DATA='%s', 总字节=%d\n", 
			clientID, i+1, dataSize, data, len(packet))
		
		// 发送消息并重试
		for retry := 0; retry < 5; retry++ {
			n, err := kcpConn.Write(packet)
			if err != nil {
				log.Printf("[Client %d] 握手消息 #%d 发送失败 (尝试 %d/5): %v\n", clientID, i+1, retry+1, err)
				time.Sleep(100 * time.Millisecond)
				continue
			}
			
			if n != len(packet) {
				log.Printf("[Client %d] 警告: 握手消息 #%d 只发送了 %d/%d 字节\n", clientID, i+1, n, len(packet))
				time.Sleep(50 * time.Millisecond)
				continue
			}
			
			log.Printf("[Client %d] 握手消息 #%d 发送成功: 发送了 %d 字节\n", clientID, i+1, n)
			sentSuccess = true
			break
		}
		
		// 确保每个消息都有时间被处理
		time.Sleep(300 * time.Millisecond)
	}
	
	if !sentSuccess {
		log.Printf("[Client %d] 所有握手消息都发送失败，终止测试\n", clientID)
		return
	}
	
	// 等待服务端处理
	log.Printf("[Client %d] 握手消息已发送，等待服务端处理...\n", clientID)
	time.Sleep(1 * time.Second)
	
	// 发送测试消息
	successCount := 0
	
	for i := 0; i < numMessages; i++ {
		// 创建简单测试数据
		testData := fmt.Sprintf("TEST_MSG_%d", i+1)
		dataBytes := []byte(testData)
		
		// 计算数据长度
		dataSize := len(dataBytes)
		
		// 创建完整消息包: | 2B size | DATA |
		packet := make([]byte, 2+dataSize)
		
		// 设置2字节的SIZE
		binary.BigEndian.PutUint16(packet[0:2], uint16(dataSize))
		
		// 设置DATA
		copy(packet[2:], dataBytes)
		
		// 详细日志
		log.Printf("[Client %d] 发送测试消息 #%d: SIZE=%d, DATA='%s', 总字节=%d\n", 
			clientID, i+1, dataSize, testData, len(packet))
		
		// 发送消息并重试
		var sendSuccess bool
		start := time.Now()
		
		for retry := 0; retry < 5; retry++ {
			n, err := kcpConn.Write(packet)
			if err != nil {
				log.Printf("[Client %d] 测试消息 #%d 发送失败 (尝试 %d/5): %v\n", clientID, i+1, retry+1, err)
				time.Sleep(100 * time.Millisecond)
				continue
			}
			
			if n != len(packet) {
				log.Printf("[Client %d] 警告: 测试消息 #%d 只发送了 %d/%d 字节\n", clientID, i+1, n, len(packet))
				time.Sleep(50 * time.Millisecond)
				continue
			}
			
			latency := time.Since(start)
			log.Printf("[Client %d] 测试消息 #%d 发送成功: 发送了 %d 字节, 延迟=%v\n", 
				clientID, i+1, n, latency)
			
			// 更新统计信息
			mu.Lock()
			totalSent += int64(n)
			mu.Unlock()
			
			latencyMutex.Lock()
			latencies = append(latencies, latency)
			latencyMutex.Unlock()
			
			sendSuccess = true
			break
		}
		
		if sendSuccess {
			successCount++
		}
		
		// 等待确保消息被处理
		time.Sleep(300 * time.Millisecond)
	}
	
	log.Printf("[Client %d] 完成: 成功发送 %d/%d 条测试消息\n", clientID, successCount, numMessages)
	
	// 等待一段时间，确保所有数据都被服务端处理
	time.Sleep(500 * time.Millisecond)
	
	// 更新已连接客户端计数
	atomic.AddInt64(&connectedCount, 1)
}

func showResults(duration time.Duration) {
	sent := atomic.LoadInt64(&totalSent)
	received := atomic.LoadInt64(&totalReceived)
	errors := atomic.LoadInt64(&errorCount)
	connected := atomic.LoadInt64(&connectedCount)

	fmt.Println("\n===== 性能测试结果 =====")
	fmt.Printf("测试持续时间: %v\n", duration)
	fmt.Printf("客户端数量: %d (成功连接: %d)\n", *clients, connected)
	fmt.Printf("每客户端消息数: %d\n", *messages)
	fmt.Printf("消息大小: %d 字节\n", *packetSize)
	fmt.Printf("总发送数据: %.2f MB\n", float64(sent)/(1024*1024))
	fmt.Printf("总接收数据: %.2f MB\n", float64(received)/(1024*1024))
	fmt.Printf("错误数: %d\n", errors)
	fmt.Printf("吞吐量: %.2f MB/s\n", float64(sent)/(1024*1024)/duration.Seconds())
	fmt.Printf("消息速率: %.2f 消息/秒\n", float64(*clients**messages)/duration.Seconds())

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

// TestAgentClient 
func TestAgentClient() {
	// agent_client.go 
}

func main() {
	// 
	flag.Parse()

	// 
	fmt.Printf("开始测试: 连接到 %s, %d个客户端, 每个客户端发送 %d 条消息，包大小: %d 字节\n",
		*server, *clients, *messages, *packetSize)

	// 
	latencies = make([]time.Duration, 0, *clients**messages)

	// 
	var wg sync.WaitGroup
	wg.Add(*clients)

	// 
	startTime := time.Now()

	// 
	for i := 0; i < *clients; i++ {
		go func(clientID int) {
			defer wg.Done()
			runKCPTest(clientID, *messages, *packetSize, *server, *showDetail)
		}(i)

		// 
		time.Sleep(5 * time.Millisecond)
	}

	// 
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				connected := atomic.LoadInt64(&connectedCount)
				sent := atomic.LoadInt64(&totalSent)
				errors := atomic.LoadInt64(&errorCount)
				elapsed := time.Since(startTime)

				fmt.Printf("进行中 [%v]: 已连接: %d/%d 客户端, 已发送: %.2f MB, 错误: %d\n",
					elapsed.Round(time.Second),
					connected, *clients,
					float64(sent)/(1024*1024),
					errors)
			}
		}
	}()

	// 
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// 
	case <-time.After(10 * time.Minute): // 
		fmt.Println("测试超时，强制结束")
	}

	// 
	duration := time.Since(startTime)

	// 
	showResults(duration)

	// 
	TestAgentClient()
}
