package main

import (
	"backend/client"
	"backend/config"
	"backend/handlers"
	"backend/middleware"
	"backend/models"
	"backend/services"
	"backend/utils"
	"context"
	"log"
	"os"
	"reflect"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	// 加载 .env 文件
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	// 加载配置
	cfg := config.Load("")

	WSS_URL := os.Getenv("SEPOLIA_RPC_WSS_URL")
	if WSS_URL == "" {
		log.Fatal("SEPOLIA_RPC_WSS_URL is not set")
	}
	CONTRACT_ADDRESS := os.Getenv("CONTRACT_ADDRESS")
	if WSS_URL == "" {
		log.Fatal("CONTRACT_ADDRESS is not set")
	}

	db, err := gorm.Open(mysql.Open(config.GetMySQLDSN(cfg)), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect database: %v", err)
	}

	// 自动迁移
	if err := db.AutoMigrate(models.GetModels()...); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	conn, _ := ethclient.Dial(WSS_URL)
	contractAddr := common.HexToAddress(CONTRACT_ADDRESS)

	listener, _ := client.NewMultiEventListener(contractAddr, conn)

	userService := services.NewUserService(db)
	userHandler := handlers.NewUserHandler(userService, []byte(cfg.JWT.Secret))
	auctionService := services.NewAuctionService(db, userService, nil)
	// 注册 AuctionCreated 的处理函数

	listener.RegisterHandler(reflect.TypeOf(&client.ClientAuctionCreated{}), auctionService.HandleClientAuctionCreatedEvent)

	// 注册 AuctionEnded 的处理函数
	listener.RegisterHandler(reflect.TypeOf(&client.ClientAuctionEnded{}), auctionService.HandleClientAuctionEndedEvent)

	// 注册 BidPlaced 的处理函数
	listener.RegisterHandler(reflect.TypeOf(&client.ClientBidPlaced{}), auctionService.HandleClientBidPlacedEvent)

	ctx1, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := listener.Start(ctx1); err != nil {
		log.Fatal(err)
	}
	defer listener.Stop()

	r := gin.Default()

	r.Use(middleware.Logger())
	r.Use(middleware.CORS())

	r.GET("health", func(c *gin.Context) {
		utils.Success(c, gin.H{
			"status": "ok",
		})
	})

	public := r.Group("/api/v1")
	{
		public.POST("/users/register", userHandler.Register)
		public.POST("/users/login", userHandler.Login)
	}

	// 需要认证的路由
	protected := r.Group("/api/v1")
	protected.Use(middleware.Auth([]byte(cfg.JWT.Secret)))
	{
		protected.GET("/users/me", userHandler.GetProfile)
		protected.PUT("/users/me", userHandler.UpdateProfile)
	}

	addr := cfg.Server.Host + ":" + cfg.Server.Port
	log.Printf("Server starting on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	/**
	// 之后可以通过 os.Getenv 读取变量
	rpcURL := os.Getenv("SEPOLIA_RPC_URL")
	if rpcURL == "" {
		log.Fatal("SEPOLIA_RPC_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)
	defer cancel()

	etrClient, err := ethclient.DialContext(ctx, rpcURL)
	if err != nil {
		log.Fatalf("failed to connect to Ethereum node: %v", err)
	}
	defer etrClient.Close()

	chainID, err := etrClient.ChainID(ctx)
	if err != nil {
		log.Fatalf("failed to get chain id: %v", err)
	}

	header, err := etrClient.HeaderByNumber(ctx, nil)
	if err != nil {
		log.Fatalf("failed to get latest block header: %v", err)
	}

	fmt.Println("=== Ethereum Node Info ===")
	fmt.Printf("RPC URL       : %s\n", rpcURL)
	fmt.Printf("Chain ID      : %s\n", chainID.String())
	fmt.Println("\n⚠️  注意: 'Latest' 区块是节点当前认为的最新区块，可能尚未被所有节点确认")
	fmt.Println("   不同RPC节点可能返回不同的 'latest' 区块，导致与浏览器不匹配")
	fmt.Println("   建议对比 'Safe' 或 'Finalized' 区块（已确认的区块）")
	fmt.Println()
	fmt.Printf("Latest Block  : %d\n", header.Number.Uint64())
	fmt.Printf("Block Hash    : %s\n", header.Hash().Hex())
	fmt.Printf("Block Time    : %s\n", time.Unix(int64(header.Time), 0).Format(time.RFC3339))
	fmt.Println("==========================")


	*/

	// 阻塞主线程
	//select {}
}
