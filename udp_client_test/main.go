package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/xtaci/kcp-go"
)

var (
	server     = flag.String("server", "localhost:8009", "服务器地址")
	clients    = flag.Int("clients", 100, "并发客户端数量")
	messages   = flag.Int("messages", 1000, "每个客户端发送的消息数量")
	packetSize = flag.Int("size", 1024, "每个数据包的大小(字节)")
	interval   = flag.Duration("interval", 10*time.Millisecond, "发送消息的间隔时间")
	showDetail = flag.Bool("detail", false, "显示详细信息")
	timeout    = flag.Duration("timeout", 5*time.Second, "连接超时时间")
	
	totalSent     int64
	totalReceived int64
	errorCount    int64
	connectedCount int64
	latencies     []time.Duration
	latencyMutex  sync.Mutex
)

func main() {
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
			runClient(clientID)
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

func runClient(clientID int) {
	// 添加一些错误处理和重试逻辑
	var conn *kcp.UDPSession
	var err error
	
	// 尝试连接
	conn, err = kcp.DialWithOptions(*server, nil, 10, 3)
	if err != nil {
		log.Printf("[Client %d] 连接失败: %v\n", clientID, err)
		atomic.AddInt64(&errorCount, 1)
		return
	}
	
	// 如果连接成功，确保最后关闭
	defer conn.Close()
	
	// 增加已连接客户端计数
	atomic.AddInt64(&connectedCount, 1)
	
	// 设置KCP连接参数，与服务器一致
	conn.SetStreamMode(true)
	conn.SetWindowSize(32, 32)
	conn.SetNoDelay(1, 20, 1, 1)
	conn.SetMtu(1280)
	
	if *showDetail {
		log.Printf("[Client %d] 已连接到服务器\n", clientID)
	}
	
	// 准备随机数据
	data := make([]byte, *packetSize)
	rand.Read(data)
	
	// 创建接收缓冲区
	recvBuf := make([]byte, 2048)
	
	// 用于接收的goroutine
	go func() {
		for {
			// 设置读取超时
			conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
			n, err := conn.Read(recvBuf)
			if err != nil {
				if err != io.EOF && !strings.Contains(err.Error(), "timeout") {
					log.Printf("[Client %d] 读取错误: %v\n", clientID, err)
				}
				return
			}
			atomic.AddInt64(&totalReceived, int64(n))
		}
	}()
	
	// 发送指定数量的消息
	successCount := 0
	for i := 0; i < *messages; i++ {
		// 按照服务器期望的格式构建完整的消息包
		// 1. 准备消息头(2字节长度) - 使用大端序表示数据长度
		header := make([]byte, 2)
		binary.BigEndian.PutUint16(header, uint16(*packetSize))
		
		// 2. 在数据中填入序列号和时间戳以便跟踪
		binary.BigEndian.PutUint32(data[0:4], uint32(i))      // 序列号
		binary.BigEndian.PutUint64(data[4:12], uint64(time.Now().UnixNano())) // 时间戳
		
		// 3. 将头部和数据组合成一个完整的消息
		packet := append(header, data...)
		
		// 4. 一次性发送完整的消息
		start := time.Now()
		_, err := conn.Write(packet)
		if err != nil {
			log.Printf("[Client %d] 发送失败: %v\n", clientID, err)
			atomic.AddInt64(&errorCount, 1)
			return
		}
		
		// 测量发送延迟
		latency := time.Since(start)
		
		latencyMutex.Lock()
		latencies = append(latencies, latency)
		latencyMutex.Unlock()
		
		atomic.AddInt64(&totalSent, int64(len(packet)))
		successCount++
		
		// 等待指定间隔
		time.Sleep(*interval)
	}
	
	if *showDetail {
		log.Printf("[Client %d] 完成发送，总共成功发送了 %d/%d 条消息\n", 
			clientID, successCount, *messages)
	}
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
	}
	
	fmt.Println("========================")
}
