package main

import (
	"log"
	"os"

	"evaluer/internal/api"
	"evaluer/internal/cache"
	"evaluer/internal/config"
	"evaluer/internal/dal"
	"evaluer/internal/db"
	"evaluer/internal/service"

	"github.com/joho/godotenv"
)

func main() {
	// Load .env if present
	_ = godotenv.Load()

	log.Println("Starting Evaluer Go Application Server...")

	database, err := db.InitDB()
	if err != nil {
		log.Fatalf("Database initialization failed: %v", err)
	}

	dalInst := dal.NewDAL(database)
	cacheMgr := cache.NewCacheManager(dalInst)
	configMgr := config.NewConfigManager(dalInst)
	calcEngine := service.NewGradeCalculator(cacheMgr, configMgr)

	// Pre-warm cache in memory
	go func() {
		log.Println("Pre-warming RAM cache in background...")
		_, err := cacheMgr.Refresh(nil)
		if err != nil {
			log.Printf("RAM Cache pre-warm error: %v", err)
		} else {
			log.Println("RAM Cache pre-warm completed successfully!")
		}
	}()

	router := api.SetupRouter(calcEngine, configMgr)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}

	log.Printf("Server listening on http://localhost:%s\n", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Server crash: %v", err)
	}
}
