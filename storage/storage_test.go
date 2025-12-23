package storage

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/wenkaler/xfreehack/model"

	"github.com/go-kit/kit/log"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

var (
	testDB *sqlx.DB
)

func TestMain(m *testing.M) {
	ctx := context.Background()

	dbName := "users"
	dbUser := "user"
	dbPassword := "password"

	postgresContainer, err := postgres.Run(ctx,
		"postgres:15-alpine",
		postgres.WithDatabase(dbName),
		postgres.WithUsername(dbUser),
		postgres.WithPassword(dbPassword),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(5*time.Second)),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to start container: %s\n", err)
		os.Exit(1)
	}

	defer func() {
		if err := postgresContainer.Terminate(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "failed to terminate container: %s\n", err)
		}
	}()

	connStr, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get connection string: %s\n", err)
		os.Exit(1)
	}

	testDB, err = sqlx.Open("pgx", connStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open db connection: %s\n", err)
		os.Exit(1)
	}

	// Apply Migrations
	if err := applyMigrations(testDB); err != nil {
		fmt.Fprintf(os.Stderr, "failed to apply migrations: %s\n", err)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

func applyMigrations(db *sqlx.DB) error {
	// Read init.sql
	initSQL, err := os.ReadFile("../migration/init.sql")
	if err != nil {
		return fmt.Errorf("failed to read init.sql: %w", err)
	}
	// Read alter_chats.sql
	alterSQL, err := os.ReadFile("../migration/alter_chats.sql")
	if err != nil {
		return fmt.Errorf("failed to read alter_chats.sql: %w", err)
	}

	// Apply init
	_, err = db.Exec(string(initSQL))
	if err != nil {
		return fmt.Errorf("failed to exec init.sql: %w", err)
	}
	// Apply alter
	_, err = db.Exec(string(alterSQL))
	if err != nil {
		return fmt.Errorf("failed to exec alter_chats.sql: %w", err)
	}

	return nil
}

func TestStorage_Categories(t *testing.T) {
	logger := log.NewNopLogger()
	s := &Storage{db: testDB, logger: logger}

	// Clean tables
	_, err := testDB.Exec("TRUNCATE TABLE categories CASCADE")
	require.NoError(t, err)

	cat := model.Category{
		Name: "Test Category",
		Slug: "test-category",
	}

	// Test Save
	id, err := s.SaveCategory(cat)
	require.NoError(t, err)
	assert.NotZero(t, id)

	// Test Get All
	cats, err := s.GetAllCategories()
	require.NoError(t, err)
	assert.Len(t, cats, 1)
	assert.Equal(t, "Test Category", cats[0].Name)
	assert.Equal(t, "test-category", cats[0].Slug)

	// Test Upsert (Update name)
	cat.Name = "Updated Name"
	id2, err := s.SaveCategory(cat)
	require.NoError(t, err)
	assert.Equal(t, id, id2) // Should be same ID

	cats, err = s.GetAllCategories()
	require.NoError(t, err)
	assert.Equal(t, "Updated Name", cats[0].Name)
}

func TestStorage_Stores(t *testing.T) {
	logger := log.NewNopLogger()
	s := &Storage{db: testDB, logger: logger}
	_, err := testDB.Exec("TRUNCATE TABLE categories, stores CASCADE")
	require.NoError(t, err)

	// Create Category
	catID, err := s.SaveCategory(model.Category{Name: "Cat1", Slug: "cat1"})
	require.NoError(t, err)

	// Test Save Store
	store := model.Store{
		Name:       "Store 1",
		Slug:       "store-1",
		URL:        "http://store1.com",
		CategoryID: catID,
	}
	id, err := s.SaveStore(store)
	require.NoError(t, err)
	assert.NotZero(t, id)

	// Test GetStore
	fetchedStore, err := s.GetStore(id)
	require.NoError(t, err)
	assert.Equal(t, "Store 1", fetchedStore.Name)
	assert.Equal(t, catID, fetchedStore.CategoryID)

	// Test GetStoresByCategory
	stores, err := s.GetStoresByCategory(catID)
	require.NoError(t, err)
	assert.Len(t, stores, 1)
}

func TestStorage_Coupons(t *testing.T) {
	logger := log.NewNopLogger()
	s := &Storage{db: testDB, logger: logger}
	_, err := testDB.Exec("TRUNCATE TABLE categories, stores, coupons CASCADE")
	require.NoError(t, err)

	catID, _ := s.SaveCategory(model.Category{Name: "Cat1", Slug: "cat1"})
	storeID, _ := s.SaveStore(model.Store{Name: "Store1", Slug: "store1", CategoryID: catID})

	coupon := model.Coupon{
		StoreID:     storeID,
		Code:        "SALE10",
		Description: "10% Off",
		ExpiryDate:  time.Now().Add(24 * time.Hour).Unix(),
		Link:        "http://link.com",
		IsExclusive: false,
	}

	// Test Save Coupon
	err = s.SaveCoupon(coupon)
	require.NoError(t, err)

	// Test LoadCollect
	m, err := s.LoadCollect()
	require.NoError(t, err)
	assert.Contains(t, m, "http://link.com")
	assert.Equal(t, "SALE10", m["http://link.com"].Code)
}

func TestStorage_NewChat(t *testing.T) {
	logger := log.NewNopLogger()
	s := &Storage{db: testDB, logger: logger}
	_, err := testDB.Exec("TRUNCATE TABLE chats CASCADE")
	require.NoError(t, err)

	chat := &tgbotapi.Chat{
		ID:        12345,
		Type:      "private",
		UserName:  "testuser",
		FirstName: "Test",
		LastName:  "User",
	}

	err = s.NewChat(chat)
	require.NoError(t, err)

	// Verify defaults
	var offset, hour int
	err = testDB.QueryRow("SELECT timezone_offset, notification_hour FROM chats WHERE id = $1", 12345).Scan(&offset, &hour)
	require.NoError(t, err)
	assert.Equal(t, 3, offset)
	assert.Equal(t, 18, hour)

	// Test Update Timezone
	err = s.SetUserTimezone(12345, 5)
	require.NoError(t, err)

	off, err := s.GetUserTimezone(12345)
	require.NoError(t, err)
	assert.Equal(t, 5, off)
}

func TestStorage_Subscriptions(t *testing.T) {
	logger := log.NewNopLogger()
	s := &Storage{db: testDB, logger: logger}
	_, err := testDB.Exec("TRUNCATE TABLE categories, stores, chats, subscriptions CASCADE")
	require.NoError(t, err)

	// Setup
	catID, _ := s.SaveCategory(model.Category{Name: "CatSub", Slug: "catsub"})
	storeID, _ := s.SaveStore(model.Store{Name: "StoreSub", Slug: "storesub", CategoryID: catID})
	chat := &tgbotapi.Chat{ID: 999, Type: "private", UserName: "user999"}
	s.NewChat(chat)

	// Test Subscribe Category
	err = s.SubscribeToCategory(999, catID)
	require.NoError(t, err)

	subs, err := s.GetSubscribedCategories(999)
	require.NoError(t, err)
	assert.Contains(t, subs, catID)

	// Test Unsubscribe Category
	err = s.UnsubscribeFromCategory(999, catID)
	require.NoError(t, err)
	subs, _ = s.GetSubscribedCategories(999)
	assert.NotContains(t, subs, catID)

	// Test Subscribe Store
	err = s.SubscribeToStore(999, storeID)
	require.NoError(t, err)

	stores, err := s.GetSubscribedStores(999)
	require.NoError(t, err)
	assert.Contains(t, stores, storeID)
}

func TestStorage_CouponDeduplication(t *testing.T) {
	logger := log.NewNopLogger()
	s := &Storage{db: testDB, logger: logger}
	_, err := testDB.Exec("TRUNCATE TABLE categories, stores, coupons, chats, relation_chat_coupons CASCADE")
	require.NoError(t, err)

	catID, _ := s.SaveCategory(model.Category{Name: "CatDedup", Slug: "catdedup"})
	storeID, _ := s.SaveStore(model.Store{Name: "StoreDedup", Slug: "storededup", CategoryID: catID})
	chatID := int64(888)
	s.NewChat(&tgbotapi.Chat{ID: chatID, Type: "private", UserName: "user888"})

	coupon := model.Coupon{
		StoreID:     storeID,
		Code:        "DEDUP1",
		Description: "Dedup Test",
		ExpiryDate:  time.Now().Add(24 * time.Hour).Unix(),
		Link:        "http://dedup.com",
	}
	s.SaveCoupon(coupon)

	// Get ID of saved coupon
	m, _ := s.LoadCollect()
	savedCoupon := m["http://dedup.com"]

	// 1. Should get coupon initially
	coupons, err := s.GetNotUseCouponCount(chatID, 10)
	require.NoError(t, err)
	assert.Len(t, coupons, 1)
	assert.Equal(t, savedCoupon.ID, coupons[0].ID)

	// 2. Mark as read
	err = s.MarkAsRead(chatID, []model.Coupon{savedCoupon})
	require.NoError(t, err)

	// 3. Should NOT get coupon again
	coupons, err = s.GetNotUseCouponCount(chatID, 10)
	require.NoError(t, err)
	assert.Len(t, coupons, 0, "Should not return marked-as-read coupons")
}

func TestStorage_CountNotUseCouponByStore(t *testing.T) {
	logger := log.NewNopLogger()
	s := &Storage{db: testDB, logger: logger}
	_, err := testDB.Exec("TRUNCATE TABLE categories, stores, coupons, chats, relation_chat_coupons CASCADE")
	require.NoError(t, err)

	catID, _ := s.SaveCategory(model.Category{Name: "CatCnt", Slug: "catcnt"})
	storeID, _ := s.SaveStore(model.Store{Name: "StoreCnt", Slug: "storecnt", CategoryID: catID})
	chatID := int64(777)
	s.NewChat(&tgbotapi.Chat{ID: chatID, Type: "private", UserName: "user777"})

	// Save 2 coupons
	c1 := model.Coupon{StoreID: storeID, Code: "C1", Description: "D1", ExpiryDate: time.Now().Add(1 * time.Hour).Unix(), Link: "http://c1.com"}
	c2 := model.Coupon{StoreID: storeID, Code: "C2", Description: "D2", ExpiryDate: time.Now().Add(1 * time.Hour).Unix(), Link: "http://c2.com"}
	s.SaveCoupon(c1)
	s.SaveCoupon(c2)

	// Verify count is 2
	cnt, err := s.CountNotUseCouponByStore(chatID, storeID)
	require.NoError(t, err)
	assert.Equal(t, 2, cnt)

	// Mark 1 as read
	// Need ID
	all, _ := s.GetNotUseCouponCount(chatID, 10)
	first := all[0]
	s.MarkAsRead(chatID, []model.Coupon{first})

	// Verify count is 1
	cnt, err = s.CountNotUseCouponByStore(chatID, storeID)
	require.NoError(t, err)
	assert.Equal(t, 1, cnt)
}
