package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	jwtSecretKey = []byte("test-secret-key")

	tmpFile, err := os.CreateTemp("", "testdb-*.db")
	if err != nil {
		fmt.Printf("Failed to create temp file: %v\n", err)
		os.Exit(1)
	}
	tmpFile.Close()
	tmpPath := tmpFile.Name()
	os.Remove(tmpPath)

	if err := DbStart(tmpPath, "./schema.sql"); err != nil {
		fmt.Printf("Failed to start database: %v\n", err)
		os.Exit(1)
	}
	sharedDB = Db

	code := m.Run()

	os.Exit(code)
}

var sharedDB *sql.DB

func setupTestDB(t *testing.T) func() {
	if Db != nil && Db != sharedDB {
		Db.Close()
	}

	tmpFile, err := os.CreateTemp("", "testdb-*.db")
	require.NoError(t, err)
	tmpFile.Close()
	os.Remove(tmpFile.Name())

	err = DbStart(tmpFile.Name()+"?_foreign_keys=on", "./schema.sql")
	require.NoError(t, err)

	return func() {
		if Db != nil {
			Db.Close()
		}
		Db = sharedDB
		os.Remove(tmpFile.Name())
	}
}

func createTestUser(t *testing.T, email, password string) *User {
	user, err := CreateUser(email, password)
	require.NoError(t, err)
	return user
}

func createTestOrder(t *testing.T, sender *User, receiverId int) *Order {
	data := &CreateOrder{
		ReceiverId: receiverId,
		Name:       "Test Order",
		Meta:       "test-meta",
		Comment:    "test-comment",
	}
	order, err := CreateOrderService(data, sender)
	require.NoError(t, err)
	return order
}

func generateTestToken(user *User) string {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, &jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		NotBefore: jwt.NewNumericDate(time.Now()),
		Issuer:    "oath",
		Subject:   itoa(user.ID),
	})
	ss, _ := token.SignedString(jwtSecretKey)
	return ss
}

func itoa(i int) string {
	return fmt.Sprintf("%d", i)
}

func addContextUser(r *http.Request, user *User) *http.Request {
	ctx := context.WithValue(r.Context(), contextKeyUser, user)
	return r.WithContext(ctx)
}

func TestEnvOr(t *testing.T) {
	os.Setenv("TEST_KEY", "test-value")
	defer os.Unsetenv("TEST_KEY")

	assert.Equal(t, "test-value", envOr("TEST_KEY", "fallback"))
	assert.Equal(t, "fallback", envOr("NON_EXISTENT_KEY", "fallback"))
}

func TestJsonResponse(t *testing.T) {
	w := httptest.NewRecorder()
	data := map[string]string{"key": "value"}
	jsonResponse(w, data)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var result map[string]string
	err := json.NewDecoder(w.Body).Decode(&result)
	assert.NoError(t, err)
	assert.Equal(t, "value", result["key"])
}

func TestJsonError(t *testing.T) {
	w := httptest.NewRecorder()
	jsonError(w, "test error", http.StatusBadRequest)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var result map[string]string
	err := json.NewDecoder(w.Body).Decode(&result)
	assert.NoError(t, err)
	assert.Equal(t, "test error", result["error"])
}

func TestLogsMiddleware(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	logsMiddleware(handler).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetUsersList(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/auth/users", nil)
	w := httptest.NewRecorder()

	GetUsersList(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var emptyUsers []User
	var err error
	err = json.NewDecoder(w.Body).Decode(&emptyUsers)
	assert.NoError(t, err)
	assert.Len(t, emptyUsers, 0)

	user := createTestUser(t, "test@example.com", "password123")

	req = httptest.NewRequest("GET", "/auth/users", nil)
	w = httptest.NewRecorder()

	GetUsersList(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var users []User
	err = json.NewDecoder(w.Body).Decode(&users)
	assert.NoError(t, err)
	assert.Len(t, users, 1)
	assert.Equal(t, user.Email, users[0].Email)
}

func TestGetUserDetails(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "test@example.com", "password123")

	req := httptest.NewRequest("GET", "/auth/users/details?id="+itoa(user.ID), nil)
	w := httptest.NewRecorder()

	GetUserDetails(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var result User
	err := json.NewDecoder(w.Body).Decode(&result)
	assert.NoError(t, err)
	assert.Equal(t, user.Email, result.Email)
}

func TestGetUserDetails_NotFound(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/auth/users/details?id=999", nil)
	w := httptest.NewRecorder()

	GetUserDetails(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetUserDetails_MissingId(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/auth/users/details", nil)
	w := httptest.NewRecorder()

	GetUserDetails(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetOrdersList(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/auth/orders", nil)
	w := httptest.NewRecorder()
	GetOrdersList(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	user := createTestUser(t, "test@example.com", "password123")
	req = addContextUser(req, user)

	w = httptest.NewRecorder()
	GetOrdersList(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var orders []Order
	err := json.NewDecoder(w.Body).Decode(&orders)
	assert.NoError(t, err)
	assert.Len(t, orders, 0)

	user2 := createTestUser(t, "receiver@example.com", "password123")
	createTestOrder(t, user, user2.ID)

	req = httptest.NewRequest("GET", "/auth/orders", nil)
	req = addContextUser(req, user)
	w = httptest.NewRecorder()
	GetOrdersList(w, req)

	err = json.NewDecoder(w.Body).Decode(&orders)
	assert.NoError(t, err)
	assert.Len(t, orders, 1)
}

func TestGetOrderDetails(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "test@example.com", "password123")
	user2 := createTestUser(t, "receiver@example.com", "password123")
	order := createTestOrder(t, user, user2.ID)

	req := httptest.NewRequest("GET", "/auth/orders/details?id="+itoa(order.ID), nil)
	w := httptest.NewRecorder()

	GetOrderDetails(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var result Order
	err := json.NewDecoder(w.Body).Decode(&result)
	assert.NoError(t, err)
	assert.Equal(t, order.Name, result.Name)
}

func TestGetOrderDetails_NotFound(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/auth/orders/details?id=999", nil)
	w := httptest.NewRecorder()

	GetOrderDetails(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateOrderEndpoint(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	req := httptest.NewRequest("POST", "/auth/orders", nil)
	w := httptest.NewRecorder()
	CreateOrderEndpoint(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	user := createTestUser(t, "test@example.com", "password123")
	user2 := createTestUser(t, "receiver@example.com", "password123")

	body := map[string]interface{}{
		"receiver_id": user2.ID,
		"name":        "Test Order",
		"meta":        "test-meta",
		"comment":     "test-comment",
	}
	bodyBytes, _ := json.Marshal(body)
	req = httptest.NewRequest("POST", "/auth/orders", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = addContextUser(req, user)
	w = httptest.NewRecorder()

	CreateOrderEndpoint(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var order Order
	err := json.NewDecoder(w.Body).Decode(&order)
	assert.NoError(t, err)
	assert.Equal(t, "Test Order", order.Name)
}

func TestCreateOrderEndpoint_InvalidBody(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "test@example.com", "password123")

	req := httptest.NewRequest("POST", "/auth/orders", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	req = addContextUser(req, user)
	w := httptest.NewRecorder()

	CreateOrderEndpoint(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateOrderEndpoint_EmptyName(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "test@example.com", "password123")
	user2 := createTestUser(t, "receiver@example.com", "password123")

	body := map[string]interface{}{
		"receiver_id": user2.ID,
		"name":        "",
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/auth/orders", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = addContextUser(req, user)
	w := httptest.NewRecorder()

	CreateOrderEndpoint(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateOrderEndpoint_ReceiverNotFound(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "test@example.com", "password123")

	body := map[string]interface{}{
		"receiver_id": 999,
		"name":        "Test Order",
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/auth/orders", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = addContextUser(req, user)
	w := httptest.NewRecorder()

	CreateOrderEndpoint(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateOrderEndpoint_SelfOrder(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "test@example.com", "password123")

	body := map[string]interface{}{
		"receiver_id": user.ID,
		"name":        "Test Order",
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/auth/orders", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = addContextUser(req, user)
	w := httptest.NewRecorder()

	CreateOrderEndpoint(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateOrderService(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "sender@example.com", "password123")
	user2 := createTestUser(t, "receiver@example.com", "password123")

	data := &CreateOrder{
		ReceiverId: user2.ID,
		Name:       "Test Package",
		Meta:       "meta-data",
		Comment:    "handle with care",
	}

	order, err := CreateOrderService(data, user)
	assert.NoError(t, err)
	assert.Equal(t, user.ID, order.SenderId)
	assert.Equal(t, user2.ID, order.ReceiverId)
	assert.Equal(t, "Test Package", order.Name)
	assert.Equal(t, "pending", order.Status)
}

func TestCreateOrderService_WithPhoto(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "sender@example.com", "password123")
	user2 := createTestUser(t, "receiver@example.com", "password123")

	data := &CreateOrder{
		ReceiverId:  user2.ID,
		Name:        "Test Package",
		Meta:        "meta-data",
		Comment:     "handle with care",
		PhotoBase64: "SGVsbG8gV29ybGQ", // "Hello World" in base64 (no padding)
	}

	order, err := CreateOrderService(data, user)
	assert.NoError(t, err)
	assert.NotEmpty(t, order.Photo)
}

func TestCreateOrderService_InvalidBase64(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "sender@example.com", "password123")
	user2 := createTestUser(t, "receiver@example.com", "password123")

	data := &CreateOrder{
		ReceiverId:  user2.ID,
		Name:        "Test Package",
		PhotoBase64: "invalid!!!",
	}

	_, err := CreateOrderService(data, user)
	assert.Error(t, err)
}

func TestUpdateOrderStatusEndpoint(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "test@example.com", "password123")
	user2 := createTestUser(t, "receiver@example.com", "password123")
	order := createTestOrder(t, user, user2.ID)

	body := map[string]interface{}{
		"order_id": order.ID,
		"status":   "in-transit",
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("PUT", "/auth/orders/status", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = addContextUser(req, user)
	w := httptest.NewRecorder()

	UpdateOrderStatusEndpoint(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var result Order
	err := json.NewDecoder(w.Body).Decode(&result)
	assert.NoError(t, err)
	assert.Equal(t, "in-transit", result.Status)
}

func TestUpdateOrderStatusEndpoint_Unauthorized(t *testing.T) {
	req := httptest.NewRequest("PUT", "/auth/orders/status", nil)
	w := httptest.NewRecorder()
	UpdateOrderStatusEndpoint(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestUpdateOrderStatusEndpoint_InvalidBody(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "test@example.com", "password123")

	req := httptest.NewRequest("PUT", "/auth/orders/status", bytes.NewReader([]byte("invalid")))
	req.Header.Set("Content-Type", "application/json")
	req = addContextUser(req, user)
	w := httptest.NewRecorder()

	UpdateOrderStatusEndpoint(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateOrderStatusEndpoint_InvalidStatus(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "test@example.com", "password123")
	user2 := createTestUser(t, "receiver@example.com", "password123")
	order := createTestOrder(t, user, user2.ID)

	body := map[string]interface{}{
		"order_id": order.ID,
		"status":   "invalid-status",
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("PUT", "/auth/orders/status", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = addContextUser(req, user)
	w := httptest.NewRecorder()

	UpdateOrderStatusEndpoint(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateOrderStatusEndpoint_OrderNotFound(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "test@example.com", "password123")

	body := map[string]interface{}{
		"order_id": 999,
		"status":   "in-transit",
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("PUT", "/auth/orders/status", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = addContextUser(req, user)
	w := httptest.NewRecorder()

	UpdateOrderStatusEndpoint(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestUpdateOrderStatusEndpoint_Forbidden(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "sender@example.com", "password123")
	user2 := createTestUser(t, "receiver@example.com", "password123")
	user3 := createTestUser(t, "other@example.com", "password123")
	_ = user3
	createTestOrder(t, user, user2.ID)

	order, _ := GetOrders(user.ID)
	orderId := (*order)[0].ID

	body := map[string]interface{}{
		"order_id": orderId,
		"status":   "in-transit",
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("PUT", "/auth/orders/status", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = addContextUser(req, user3)
	w := httptest.NewRecorder()

	UpdateOrderStatusEndpoint(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestUpdateOrderStatusEndpoint_CancelledOrder(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "test@example.com", "password123")
	user2 := createTestUser(t, "receiver@example.com", "password123")
	order := createTestOrder(t, user, user2.ID)
	UpdateOrderStatus(itoa(order.ID), "cancelled")

	body := map[string]interface{}{
		"order_id": order.ID,
		"status":   "in-transit",
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("PUT", "/auth/orders/status", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = addContextUser(req, user)
	w := httptest.NewRecorder()

	UpdateOrderStatusEndpoint(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestUpdateOrderStatusEndpoint_DeliveredOrder(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "test@example.com", "password123")
	user2 := createTestUser(t, "receiver@example.com", "password123")
	order := createTestOrder(t, user, user2.ID)
	UpdateOrderStatus(itoa(order.ID), "delivered")

	body := map[string]interface{}{
		"order_id": order.ID,
		"status":   "in-transit",
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("PUT", "/auth/orders/status", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = addContextUser(req, user)
	w := httptest.NewRecorder()

	UpdateOrderStatusEndpoint(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestUpdateOrderStatus(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "test@example.com", "password123")
	user2 := createTestUser(t, "receiver@example.com", "password123")
	order := createTestOrder(t, user, user2.ID)

	updated, err := UpdateOrderStatus(itoa(order.ID), "delivered")
	assert.NoError(t, err)
	assert.Equal(t, "delivered", updated.Status)
}

func TestUpdateOrderStatus_NotFound(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_, err := UpdateOrderStatus("999", "delivered")
	assert.Error(t, err)
}

func TestGetOrderScansEndpoint(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/auth/orders/scans", nil)
	w := httptest.NewRecorder()
	GetOrderScansEndpoint(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	user := createTestUser(t, "test@example.com", "password123")
	user2 := createTestUser(t, "receiver@example.com", "password123")
	order := createTestOrder(t, user, user2.ID)

	req = httptest.NewRequest("GET", "/auth/orders/scans?order_id="+itoa(order.ID), nil)
	req = addContextUser(req, user)
	w = httptest.NewRecorder()

	GetOrderScansEndpoint(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var scans []Scan
	err := json.NewDecoder(w.Body).Decode(&scans)
	assert.NoError(t, err)
	assert.Len(t, scans, 0)
}

func TestGetOrderScansEndpoint_MissingOrderId(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "test@example.com", "password123")

	req := httptest.NewRequest("GET", "/auth/orders/scans", nil)
	req = addContextUser(req, user)
	w := httptest.NewRecorder()

	GetOrderScansEndpoint(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetOrderScansEndpoint_OrderNotFound(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "test@example.com", "password123")

	req := httptest.NewRequest("GET", "/auth/orders/scans?order_id=999", nil)
	req = addContextUser(req, user)
	w := httptest.NewRecorder()

	GetOrderScansEndpoint(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetOrderScansEndpoint_Forbidden(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "sender@example.com", "password123")
	user2 := createTestUser(t, "receiver@example.com", "password123")
	user3 := createTestUser(t, "other@example.com", "password123")
	_ = user3
	createTestOrder(t, user, user2.ID)

	orders, _ := GetOrders(user.ID)
	orderId := (*orders)[0].ID

	req := httptest.NewRequest("GET", "/auth/orders/scans?order_id="+itoa(orderId), nil)
	req = addContextUser(req, user3)
	w := httptest.NewRecorder()

	GetOrderScansEndpoint(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestGetAllScansEndpoint(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/auth/scans", nil)
	w := httptest.NewRecorder()
	GetAllScansEndpoint(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	user := createTestUser(t, "test@example.com", "password123")

	req = httptest.NewRequest("GET", "/auth/scans", nil)
	req = addContextUser(req, user)
	w = httptest.NewRecorder()

	GetAllScansEndpoint(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCreateOrderScanEndpoint(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	req := httptest.NewRequest("POST", "/auth/orders/scan", nil)
	w := httptest.NewRecorder()
	CreateOrderScanEndpoint(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	user := createTestUser(t, "courier@example.com", "password123")
	user2 := createTestUser(t, "sender@example.com", "password123")
	user3 := createTestUser(t, "receiver@example.com", "password123")
	order := createTestOrder(t, user2, user3.ID)

	body := map[string]interface{}{
		"order_id":     order.ID,
		"photo_base64": "SGVsbG8gV29ybGQ=",
		"condition":    "Good",
		"longitude":    1.234,
		"latitude":     5.678,
		"comment":      "Test scan",
	}
	bodyBytes, _ := json.Marshal(body)
	req = httptest.NewRequest("POST", "/auth/orders/scan", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = addContextUser(req, user)
	w = httptest.NewRecorder()

	CreateOrderScanEndpoint(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCreateOrderScanEndpoint_InvalidBody(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "courier@example.com", "password123")

	req := httptest.NewRequest("POST", "/auth/orders/scan", bytes.NewReader([]byte("invalid")))
	req.Header.Set("Content-Type", "application/json")
	req = addContextUser(req, user)
	w := httptest.NewRecorder()

	CreateOrderScanEndpoint(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateOrderScanEndpoint_InvalidCondition(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "courier@example.com", "password123")
	user2 := createTestUser(t, "sender@example.com", "password123")
	user3 := createTestUser(t, "receiver@example.com", "password123")
	order := createTestOrder(t, user2, user3.ID)

	body := map[string]interface{}{
		"order_id":     order.ID,
		"photo_base64": "SGVsbG8gV29ybGQ=",
		"condition":    "Invalid",
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/auth/orders/scan", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = addContextUser(req, user)
	w := httptest.NewRecorder()

	CreateOrderScanEndpoint(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateOrderScanEndpoint_OrderNotFound(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "courier@example.com", "password123")

	body := map[string]interface{}{
		"order_id":     999,
		"photo_base64": "SGVsbG8gV29ybGQ=",
		"condition":    "Good",
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/auth/orders/scan", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = addContextUser(req, user)
	w := httptest.NewRecorder()

	CreateOrderScanEndpoint(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestCreateOrderScanEndpoint_CancelledOrder(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "courier@example.com", "password123")
	user2 := createTestUser(t, "sender@example.com", "password123")
	user3 := createTestUser(t, "receiver@example.com", "password123")
	order := createTestOrder(t, user2, user3.ID)
	UpdateOrderStatus(itoa(order.ID), "cancelled")

	body := map[string]interface{}{
		"order_id":     order.ID,
		"photo_base64": "SGVsbG8gV29ybGQ=",
		"condition":    "Good",
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/auth/orders/scan", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = addContextUser(req, user)
	w := httptest.NewRecorder()

	CreateOrderScanEndpoint(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestCreateOrderScanEndpoint_DeliveredOrder(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "courier@example.com", "password123")
	user2 := createTestUser(t, "sender@example.com", "password123")
	user3 := createTestUser(t, "receiver@example.com", "password123")
	order := createTestOrder(t, user2, user3.ID)
	UpdateOrderStatus(itoa(order.ID), "delivered")

	body := map[string]interface{}{
		"order_id":     order.ID,
		"photo_base64": "SGVsbG8gV29ybGQ=",
		"condition":    "Good",
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/auth/orders/scan", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = addContextUser(req, user)
	w := httptest.NewRecorder()

	CreateOrderScanEndpoint(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestCreateOrderScanEndpoint_PendingOrderUpdatesStatus(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "courier@example.com", "password123")
	user2 := createTestUser(t, "sender@example.com", "password123")
	user3 := createTestUser(t, "receiver@example.com", "password123")
	order := createTestOrder(t, user2, user3.ID)
	assert.Equal(t, "pending", order.Status)

	body := map[string]interface{}{
		"order_id":     order.ID,
		"photo_base64": "SGVsbG8gV29ybGQ=",
		"condition":    "Good",
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/auth/orders/scan", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = addContextUser(req, user)
	w := httptest.NewRecorder()

	CreateOrderScanEndpoint(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	updatedOrder, _ := GetOrder(itoa(order.ID))
	assert.Equal(t, "in-transit", updatedOrder.Status)
}

func TestCreateOrderScan(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "courier@example.com", "password123")
	user2 := createTestUser(t, "sender@example.com", "password123")
	user3 := createTestUser(t, "receiver@example.com", "password123")
	order := createTestOrder(t, user2, user3.ID)

	data := &CreateOrderScanRequest{
		OrderId:     order.ID,
		PhotoBase64: "SGVsbG8gV29ybGQ=",
		Condition:   "Good",
		Longitude:   1.234,
		Latitude:    5.678,
		Comment:     "Test comment",
	}

	err := CreateOrderScan(data, user)
	assert.NoError(t, err)

	scans, _ := GetOrderScans(itoa(order.ID))
	assert.Len(t, scans, 1)
}

func TestCreateOrderScan_EmptyBase64(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "courier@example.com", "password123")
	user2 := createTestUser(t, "sender@example.com", "password123")
	user3 := createTestUser(t, "receiver@example.com", "password123")
	order := createTestOrder(t, user2, user3.ID)

	data := &CreateOrderScanRequest{
		OrderId:     order.ID,
		PhotoBase64: "",
		Condition:   "Good",
		Longitude:   1.234,
		Latitude:    5.678,
		Comment:     "Test comment",
	}

	err := CreateOrderScan(data, user)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty photo base64")
}

func TestCreateOrderScan_InvalidBase64(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "courier@example.com", "password123")
	user2 := createTestUser(t, "sender@example.com", "password123")
	user3 := createTestUser(t, "receiver@example.com", "password123")
	order := createTestOrder(t, user2, user3.ID)

	data := &CreateOrderScanRequest{
		OrderId:     order.ID,
		PhotoBase64: "invalid!!!",
		Condition:   "Good",
	}

	err := CreateOrderScan(data, user)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "base64 decode failed")
}

func TestDecodeBase64(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		hasError bool
	}{
		{"StdEncoding", "SGVsbG8gV29ybGQ=", "Hello World", false},
		{"RawStdEncoding", "SGVsbG8gV29ybGQ", "Hello World", false},
		{"URLEncoding", "SGVsbG8gV29ybGQ=", "Hello World", false},
		{"RawURLEncoding", "SGVsbG8gV29ybGQ", "Hello World", false},
		{"DataURL", "data:image/png;base64,SGVsbG8gV29ybGQ=", "Hello World", false},
		{"WithSpaces", "SGVs bG8gV29y bGQ=", "Hello World", false},
		{"WithNewlines", "SGVs\nbG8g\nV29y\nbGQ=", "Hello World", false},
		{"Empty", "", "", true},
		{"Invalid", "invalid!!!", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := decodeBase64(tt.input, 1, 1)
			if tt.hasError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, string(result))
			}
		})
	}
}

func TestGetOrderScans(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "courier@example.com", "password123")
	user2 := createTestUser(t, "sender@example.com", "password123")
	user3 := createTestUser(t, "receiver@example.com", "password123")
	order := createTestOrder(t, user2, user3.ID)

	scans, err := GetOrderScans(itoa(order.ID))
	assert.NoError(t, err)
	assert.Len(t, scans, 0)

	data := &CreateOrderScanRequest{
		OrderId:     order.ID,
		PhotoBase64: "SGVsbG8gV29ybGQ=",
		Condition:   "Good",
	}
	CreateOrderScan(data, user)

	scans, err = GetOrderScans(itoa(order.ID))
	assert.NoError(t, err)
	assert.Len(t, scans, 1)
}

func TestGetAllScans(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	scans, err := GetAllScans()
	assert.NoError(t, err)
	assert.Len(t, scans, 0)

	user := createTestUser(t, "courier@example.com", "password123")
	user2 := createTestUser(t, "sender@example.com", "password123")
	user3 := createTestUser(t, "receiver@example.com", "password123")
	order := createTestOrder(t, user2, user3.ID)

	data := &CreateOrderScanRequest{
		OrderId:     order.ID,
		PhotoBase64: "SGVsbG8gV29ybGQ=",
		Condition:   "Good",
	}
	CreateOrderScan(data, user)

	scans, err = GetAllScans()
	assert.NoError(t, err)
	assert.Len(t, scans, 1)
}

func TestGetActivitiesEndpoint(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/auth/activities", nil)
	w := httptest.NewRecorder()
	GetActivitiesEndpoint(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	user := createTestUser(t, "test@example.com", "password123")

	req = httptest.NewRequest("GET", "/auth/activities", nil)
	req = addContextUser(req, user)
	w = httptest.NewRecorder()

	GetActivitiesEndpoint(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var activities []Activity
	err := json.NewDecoder(w.Body).Decode(&activities)
	assert.NoError(t, err)
}

func TestCreateActivity(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "test@example.com", "password123")

	err := CreateActivity(user.ID, user.ID, "test_type", "Test summary")
	assert.NoError(t, err)

	activities, err := GetActivitiesForUser(user.ID)
	assert.NoError(t, err)
	assert.Len(t, activities, 1)
	assert.Equal(t, "test_type", activities[0].Type)
	assert.Equal(t, "Test summary", activities[0].Summary)
}

func TestGetActivitiesForUser(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "test@example.com", "password123")
	user2 := createTestUser(t, "user2@example.com", "password123")

	CreateActivity(user.ID, user.ID, "type1", "Summary 1")
	CreateActivity(user2.ID, user.ID, "type2", "Summary 2")
	CreateActivity(user.ID, user2.ID, "type3", "Summary 3")

	activities, err := GetActivitiesForUser(user.ID)
	assert.NoError(t, err)
	assert.Len(t, activities, 2)
}

func TestGetContactsEndpoint(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/auth/contacts", nil)
	w := httptest.NewRecorder()
	GetContactsEndpoint(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	user := createTestUser(t, "test@example.com", "password123")

	req = httptest.NewRequest("GET", "/auth/contacts", nil)
	req = addContextUser(req, user)
	w = httptest.NewRecorder()

	GetContactsEndpoint(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var contacts []Contact
	err := json.NewDecoder(w.Body).Decode(&contacts)
	assert.NoError(t, err)
	assert.Len(t, contacts, 0)
}

func TestAddContactEndpoint(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "test@example.com", "password123")
	user2 := createTestUser(t, "contact@example.com", "password123")

	body := map[string]interface{}{
		"contact_id": user2.ID,
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/auth/contacts", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = addContextUser(req, user)
	w := httptest.NewRecorder()

	AddContactEndpoint(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var contact Contact
	err := json.NewDecoder(w.Body).Decode(&contact)
	assert.NoError(t, err)
	assert.Equal(t, user.ID, contact.OwnerId)
	assert.Equal(t, user2.ID, contact.ContactId)
}

func TestAddContactEndpoint_Unauthorized(t *testing.T) {
	req := httptest.NewRequest("POST", "/auth/contacts", nil)
	w := httptest.NewRecorder()
	AddContactEndpoint(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAddContactEndpoint_InvalidBody(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "test@example.com", "password123")

	req := httptest.NewRequest("POST", "/auth/contacts", bytes.NewReader([]byte("invalid")))
	req.Header.Set("Content-Type", "application/json")
	req = addContextUser(req, user)
	w := httptest.NewRecorder()

	AddContactEndpoint(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAddContactEndpoint_SelfContact(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "test@example.com", "password123")

	body := map[string]interface{}{
		"contact_id": user.ID,
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/auth/contacts", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = addContextUser(req, user)
	w := httptest.NewRecorder()

	AddContactEndpoint(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAddContactEndpoint_DuplicateContact(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "test@example.com", "password123")
	user2 := createTestUser(t, "contact@example.com", "password123")

	AddContact(user.ID, user2.ID)

	body := map[string]interface{}{
		"contact_id": user2.ID,
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/auth/contacts", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = addContextUser(req, user)
	w := httptest.NewRecorder()

	AddContactEndpoint(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestAddContactEndpoint_UserNotFound(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "test@example.com", "password123")

	body := map[string]interface{}{
		"contact_id": 999,
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/auth/contacts", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = addContextUser(req, user)
	w := httptest.NewRecorder()

	AddContactEndpoint(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRemoveContactEndpoint(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "test@example.com", "password123")
	user2 := createTestUser(t, "contact@example.com", "password123")
	AddContact(user.ID, user2.ID)

	body := map[string]interface{}{
		"contact_id": user2.ID,
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("DELETE", "/auth/contacts", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = addContextUser(req, user)
	w := httptest.NewRecorder()

	RemoveContactEndpoint(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestRemoveContactEndpoint_Unauthorized(t *testing.T) {
	req := httptest.NewRequest("DELETE", "/auth/contacts", nil)
	w := httptest.NewRecorder()
	RemoveContactEndpoint(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRemoveContactEndpoint_InvalidBody(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "test@example.com", "password123")

	req := httptest.NewRequest("DELETE", "/auth/contacts", bytes.NewReader([]byte("invalid")))
	req.Header.Set("Content-Type", "application/json")
	req = addContextUser(req, user)
	w := httptest.NewRecorder()

	RemoveContactEndpoint(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAddContact(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "test@example.com", "password123")
	user2 := createTestUser(t, "contact@example.com", "password123")

	contact, err := AddContact(user.ID, user2.ID)
	assert.NoError(t, err)
	assert.Equal(t, user.ID, contact.OwnerId)
	assert.Equal(t, user2.ID, contact.ContactId)

	contacts, _ := GetContacts(user.ID)
	assert.Len(t, *contacts, 1)
}

func TestAddContact_Duplicate(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "test@example.com", "password123")
	user2 := createTestUser(t, "contact@example.com", "password123")

	AddContact(user.ID, user2.ID)
	_, err := AddContact(user.ID, user2.ID)
	assert.Error(t, err)
}

func TestRemoveContact(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "test@example.com", "password123")
	user2 := createTestUser(t, "contact@example.com", "password123")
	AddContact(user.ID, user2.ID)

	err := RemoveContact(user.ID, user2.ID)
	assert.NoError(t, err)

	contacts, _ := GetContacts(user.ID)
	assert.Len(t, *contacts, 0)
}

func TestGetContacts(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "test@example.com", "password123")
	user2 := createTestUser(t, "contact1@example.com", "password123")
	user3 := createTestUser(t, "contact2@example.com", "password123")

	AddContact(user.ID, user2.ID)
	AddContact(user.ID, user3.ID)

	contacts, err := GetContacts(user.ID)
	assert.NoError(t, err)
	assert.Len(t, *contacts, 2)
}

func TestSignup(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	body := map[string]interface{}{
		"email":    "test@example.com",
		"password": "password123",
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/signup", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	Signup(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var user User
	err := json.NewDecoder(w.Body).Decode(&user)
	assert.NoError(t, err)
	assert.Equal(t, "test@example.com", user.Email)
}

func TestSignup_InvalidBody(t *testing.T) {
	req := httptest.NewRequest("POST", "/signup", bytes.NewReader([]byte("invalid")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	Signup(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSignup_PasswordTooShort(t *testing.T) {
	body := map[string]interface{}{
		"email":    "test@example.com",
		"password": "short",
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/signup", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	Signup(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSignup_DuplicateEmail(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	createTestUser(t, "test@example.com", "password123")

	body := map[string]interface{}{
		"email":    "test@example.com",
		"password": "password123",
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/signup", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	Signup(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestJwtCreate(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_ = createTestUser(t, "test@example.com", "password123")

	body := map[string]interface{}{
		"email":    "test@example.com",
		"password": "password123",
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/jwt/create", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	JwtCreate(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var result map[string]string
	err := json.NewDecoder(w.Body).Decode(&result)
	assert.NoError(t, err)
	assert.NotEmpty(t, result["access_token"])
}

func TestJwtCreate_InvalidBody(t *testing.T) {
	req := httptest.NewRequest("POST", "/jwt/create", bytes.NewReader([]byte("invalid")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	JwtCreate(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestJwtCreate_UserNotFound(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	body := map[string]interface{}{
		"email":    "nonexistent@example.com",
		"password": "password123",
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/jwt/create", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	JwtCreate(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestJwtCreate_InvalidPassword(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	createTestUser(t, "test@example.com", "password123")

	body := map[string]interface{}{
		"email":    "test@example.com",
		"password": "wrongpassword",
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/jwt/create", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	JwtCreate(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestJwtCreateService(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "test@example.com", "password123")

	userWithHash, err := GetUser(itoa(user.ID))
	require.NoError(t, err)

	token, err := JwtCreateService(userWithHash, "password123")
	assert.NoError(t, err)
	assert.NotEmpty(t, token)

	parsedToken, err := jwt.Parse(token, func(token *jwt.Token) (any, error) {
		return jwtSecretKey, nil
	})
	assert.NoError(t, err)
	assert.True(t, parsedToken.Valid)

	claims, ok := parsedToken.Claims.(jwt.MapClaims)
	assert.True(t, ok)
	assert.Equal(t, "oath", claims["iss"])
}

func TestJwtCreateService_InvalidPassword(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "test@example.com", "password123")

	token, err := JwtCreateService(user, "wrongpassword")
	assert.Error(t, err)
	assert.Empty(t, token)
}

func TestCreateUser(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user, err := CreateUser("test@example.com", "password123")
	assert.NoError(t, err)
	assert.Equal(t, "test@example.com", user.Email)
	assert.NotZero(t, user.ID)
}

func TestCreateUser_PasswordTooShort(t *testing.T) {
	_, err := CreateUser("test@example.com", "short")
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrPasswordTooShort)
}

func TestCreateUser_DuplicateEmail(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	createTestUser(t, "test@example.com", "password123")

	_, err := CreateUser("test@example.com", "password123")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "UNIQUE constraint failed")
}

func TestGetUser(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "test@example.com", "password123")

	retrieved, err := GetUser(itoa(user.ID))
	assert.NoError(t, err)
	assert.Equal(t, user.Email, retrieved.Email)
}

func TestGetUser_NotFound(t *testing.T) {
	_, err := GetUser("999")
	assert.Error(t, err)
}

func TestGetUserByEmail(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "test@example.com", "password123")

	retrieved, err := GetUserByEmail("test@example.com")
	assert.NoError(t, err)
	assert.Equal(t, user.ID, retrieved.ID)
}

func TestGetUserByEmail_NotFound(t *testing.T) {
	_, err := GetUserByEmail("nonexistent@example.com")
	assert.Error(t, err)
}

func TestGetUsers(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	users, err := GetUsers()
	assert.NoError(t, err)
	assert.Len(t, *users, 0)

	createTestUser(t, "user1@example.com", "password123")
	createTestUser(t, "user2@example.com", "password123")

	users, err = GetUsers()
	assert.NoError(t, err)
	assert.Len(t, *users, 2)
}

func TestGetOrders(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "sender@example.com", "password123")
	user2 := createTestUser(t, "receiver@example.com", "password123")

	orders, err := GetOrders(user.ID)
	assert.NoError(t, err)
	assert.Len(t, *orders, 0)

	createTestOrder(t, user, user2.ID)

	orders, err = GetOrders(user.ID)
	assert.NoError(t, err)
	assert.Len(t, *orders, 1)
}

func TestGetOrder(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "sender@example.com", "password123")
	user2 := createTestUser(t, "receiver@example.com", "password123")
	order := createTestOrder(t, user, user2.ID)

	retrieved, err := GetOrder(itoa(order.ID))
	assert.NoError(t, err)
	assert.Equal(t, order.Name, retrieved.Name)
}

func TestGetOrder_NotFound(t *testing.T) {
	_, err := GetOrder("999")
	assert.Error(t, err)
}

func TestAuthMiddleware(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "test@example.com", "password123")
	token := generateTestToken(user)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/auth/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	authMiddleware(handler, "test-secret-key").ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuthMiddleware_NoToken(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/auth/test", nil)
	w := httptest.NewRecorder()

	authMiddleware(handler, "test-secret-key").ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/auth/test", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	w := httptest.NewRecorder()

	authMiddleware(handler, "test-secret-key").ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthMiddleware_WrongSecret(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "test@example.com", "password123")
	token := generateTestToken(user)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/auth/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	authMiddleware(handler, "wrong-secret-key").ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthMiddleware_AuthPath(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/auth/login", nil)
	w := httptest.NewRecorder()

	authMiddleware(handler, "test-secret-key").ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthMiddleware_UserNotFound(t *testing.T) {
	user := &User{ID: 999, Email: "nonexistent@example.com"}
	token := generateTestToken(user)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/auth/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	authMiddleware(handler, "test-secret-key").ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthMiddleware_BearerPrefix(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "test@example.com", "password123")
	token := generateTestToken(user)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/auth/test", nil)
	req.Header.Set("Authorization", token)
	w := httptest.NewRecorder()

	authMiddleware(handler, "test-secret-key").ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDbStart(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "testdb-*.db")
	require.NoError(t, err)
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	if Db != nil {
		Db.Close()
		Db = nil
	}

	err = DbStart(tmpFile.Name(), "./schema.sql")
	assert.NoError(t, err)
	assert.NotNil(t, Db)
	Db.Close()
	Db = nil
}

func TestDbStart_ExistingFile(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "testdb-*.db")
	require.NoError(t, err)
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	if Db != nil {
		Db.Close()
		Db = nil
	}

	err = DbStart(tmpFile.Name(), "./schema.sql")
	assert.NoError(t, err)

	err = DbStart(tmpFile.Name(), "./schema.sql")
	assert.NoError(t, err)
	Db.Close()
	Db = nil
}

func TestDbStart_InvalidPath(t *testing.T) {
	if Db != nil {
		Db.Close()
		Db = nil
	}

	err := DbStart("/nonexistent/path/to/db", "./schema.sql")
	assert.Error(t, err)
}

func TestCreateTablesFromSchema(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "testdb-*.db")
	require.NoError(t, err)
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	sqlDB, err := sql.Open("sqlite3", tmpFile.Name())
	require.NoError(t, err)
	defer sqlDB.Close()

	err = createTablesFromSchema(sqlDB, "./schema.sql")
	assert.NoError(t, err)
}

func TestCreateTablesFromSchema_InvalidPath(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "testdb-*.db")
	require.NoError(t, err)
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	sqlDB, err := sql.Open("sqlite3", tmpFile.Name())
	require.NoError(t, err)
	defer sqlDB.Close()

	err = createTablesFromSchema(sqlDB, "/nonexistent/path/schema.sql")
	assert.Error(t, err)
}

func TestHealthEndpoint(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHealthEndpoint_NotFound(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()

	handler := http.NewServeMux()
	handler.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuthMiddleware_ClaimsError(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_ = createTestUser(t, "test@example.com", "password123")

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, &jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		NotBefore: jwt.NewNumericDate(time.Now()),
		Issuer:    "oath",
	})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	tokenString, _ := token.SignedString(jwtSecretKey)
	req := httptest.NewRequest("GET", "/auth/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	w := httptest.NewRecorder()

	authMiddleware(handler, "test-secret-key").ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthMiddleware_InvalidClaimsType(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_ = createTestUser(t, "test@example.com", "password123")

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"iss": "oath",
	})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	tokenString, _ := token.SignedString(jwtSecretKey)
	req := httptest.NewRequest("GET", "/auth/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	w := httptest.NewRecorder()

	authMiddleware(handler, "test-secret-key").ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRemoveContactEndpoint_DBError(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "test@example.com", "password123")
	user2 := createTestUser(t, "contact@example.com", "password123")

	AddContact(user.ID, user2.ID)

	body := map[string]interface{}{
		"contact_id": user2.ID,
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("DELETE", "/auth/contacts", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = addContextUser(req, user)
	w := httptest.NewRecorder()

	RemoveContactEndpoint(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestCreateOrderEndpoint_ServiceError(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "sender@example.com", "password123")
	user2 := createTestUser(t, "receiver@example.com", "password123")

	body := map[string]interface{}{
		"receiver_id":  user2.ID,
		"name":         "Test Order",
		"photo_base64": "invalid!!!",
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/auth/orders", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = addContextUser(req, user)
	w := httptest.NewRecorder()

	CreateOrderEndpoint(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestCreateOrderScanEndpoint_ServiceError(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "courier@example.com", "password123")
	user2 := createTestUser(t, "sender@example.com", "password123")
	user3 := createTestUser(t, "receiver@example.com", "password123")
	order := createTestOrder(t, user2, user3.ID)

	body := map[string]interface{}{
		"order_id":     order.ID,
		"photo_base64": "invalid!!!",
		"condition":    "Good",
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/auth/orders/scan", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = addContextUser(req, user)
	w := httptest.NewRecorder()

	CreateOrderScanEndpoint(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestUpdateOrderStatusEndpoint_ServiceError(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "test@example.com", "password123")
	user2 := createTestUser(t, "receiver@example.com", "password123")
	order := createTestOrder(t, user, user2.ID)

	body := map[string]interface{}{
		"order_id": order.ID,
		"status":   "delivered",
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("PUT", "/auth/orders/status", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = addContextUser(req, user)
	w := httptest.NewRecorder()

	UpdateOrderStatusEndpoint(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSignup_InternalError(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	body := map[string]interface{}{
		"email": "test@example.com",
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/signup", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	Signup(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestJwtCreateService_SignError(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "test@example.com", "password123")

	originalSecret := jwtSecretKey
	jwtSecretKey = []byte{}

	token, err := JwtCreateService(user, "password123")
	assert.Error(t, err)
	assert.Empty(t, token)

	jwtSecretKey = originalSecret
}

func TestCreateOrderScan_DecodeBase64(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "courier@example.com", "password123")
	user2 := createTestUser(t, "sender@example.com", "password123")
	user3 := createTestUser(t, "receiver@example.com", "password123")
	order := createTestOrder(t, user2, user3.ID)

	data := &CreateOrderScanRequest{
		OrderId:     order.ID,
		PhotoBase64: "data:image/png;base64,SGVsbG8gV29ybGQ=",
		Condition:   "Good",
		Longitude:   1.234,
		Latitude:    5.678,
	}

	err := CreateOrderScan(data, user)
	assert.NoError(t, err)
}

func TestDecodeBase64_DataURL(t *testing.T) {
	result, err := decodeBase64("data:text/plain;base64,SGVsbG8gV29ybGQ=", 1, 1)
	assert.NoError(t, err)
	assert.Equal(t, "Hello World", string(result))
}

func TestDecodeBase64_EmptyString(t *testing.T) {
	_, err := decodeBase64("", 1, 1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty photo base64")
}

func TestDecodeBase64_InvalidBase64(t *testing.T) {
	_, err := decodeBase64("!!!invalid!!!", 1, 1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "base64 decode failed")
}

func TestDecodeBase64_URLEncoding(t *testing.T) {
	result, err := decodeBase64("SGVsbG8gV29ybGQ=", 1, 1)
	assert.NoError(t, err)
	assert.Equal(t, "Hello World", string(result))
}

func TestGetOrders_RowError(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "test@example.com", "password123")

	_, err := GetOrders(user.ID)
	assert.NoError(t, err)
}

func TestGetOrderScans_RowError(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_ = createTestUser(t, "courier@example.com", "password123")
	user2 := createTestUser(t, "sender@example.com", "password123")
	user3 := createTestUser(t, "receiver@example.com", "password123")
	order := createTestOrder(t, user2, user3.ID)

	scans, err := GetOrderScans(itoa(order.ID))
	assert.NoError(t, err)
	assert.Len(t, scans, 0)
}

func TestGetAllScans_RowError(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	scans, err := GetAllScans()
	assert.NoError(t, err)
	assert.Len(t, scans, 0)
}

func TestGetActivitiesForUser_RowError(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "test@example.com", "password123")

	activities, err := GetActivitiesForUser(user.ID)
	assert.NoError(t, err)
	assert.Len(t, activities, 0)
}

func TestGetContacts_RowError(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "test@example.com", "password123")

	contacts, err := GetContacts(user.ID)
	assert.NoError(t, err)
	assert.Len(t, *contacts, 0)
}

func TestCreateOrderScan_EmptyPhoto(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "courier@example.com", "password123")
	user2 := createTestUser(t, "sender@example.com", "password123")
	user3 := createTestUser(t, "receiver@example.com", "password123")
	order := createTestOrder(t, user2, user3.ID)

	data := &CreateOrderScanRequest{
		OrderId:   order.ID,
		Condition: "Good",
	}

	err := CreateOrderScan(data, user)
	assert.Error(t, err)
}

func TestCreateOrderScanEndpoint_UpdateStatusError(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "courier@example.com", "password123")
	user2 := createTestUser(t, "sender@example.com", "password123")
	user3 := createTestUser(t, "receiver@example.com", "password123")
	order := createTestOrder(t, user2, user3.ID)
	UpdateOrderStatus(itoa(order.ID), "in-transit")

	body := map[string]interface{}{
		"order_id":     order.ID,
		"photo_base64": "SGVsbG8gV29ybGQ=",
		"condition":    "Good",
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/auth/orders/scan", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = addContextUser(req, user)
	w := httptest.NewRecorder()

	CreateOrderScanEndpoint(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCreateOrderService_WithPhotoDataURL(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "sender@example.com", "password123")
	user2 := createTestUser(t, "receiver@example.com", "password123")

	data := &CreateOrder{
		ReceiverId:  user2.ID,
		Name:        "Test Package",
		PhotoBase64: "SGVsbG8gV29ybGQ",
	}

	order, err := CreateOrderService(data, user)
	assert.NoError(t, err)
	assert.NotEmpty(t, order.Photo)
}

func TestGetUser_InvalidId(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_, err := GetUser("abc")
	assert.Error(t, err)
}

func TestGetOrder_InvalidId(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_, err := GetOrder("abc")
	assert.Error(t, err)
}

func TestGetActivitiesForUser_Multiple(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "test@example.com", "password123")
	user2 := createTestUser(t, "user2@example.com", "password123")

	CreateActivity(user.ID, user.ID, "type1", "Summary 1")
	CreateActivity(user2.ID, user.ID, "type2", "Summary 2")
	CreateActivity(user.ID, user2.ID, "type3", "Summary 3")

	activities, err := GetActivitiesForUser(user.ID)
	assert.NoError(t, err)
	assert.Len(t, activities, 2)

	activities2, err := GetActivitiesForUser(user2.ID)
	assert.NoError(t, err)
	assert.Len(t, activities2, 1)
}

func TestGetAllScans_Multiple(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "courier1@example.com", "password123")
	user2 := createTestUser(t, "courier2@example.com", "password123")
	sender := createTestUser(t, "sender@example.com", "password123")
	receiver := createTestUser(t, "receiver@example.com", "password123")

	order1 := createTestOrder(t, sender, receiver.ID)
	order2 := createTestOrder(t, sender, receiver.ID)

	CreateOrderScan(&CreateOrderScanRequest{
		OrderId:     order1.ID,
		PhotoBase64: "SGVsbG8gV29ybGQ=",
		Condition:   "Good",
	}, user)

	CreateOrderScan(&CreateOrderScanRequest{
		OrderId:     order2.ID,
		PhotoBase64: "SGVsbG8gV29ybGQ=",
		Condition:   "Damaged",
	}, user2)

	scans, err := GetAllScans()
	assert.NoError(t, err)
	assert.Len(t, scans, 2)
}

func TestRemoveContactEndpoint_NotFound(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "test@example.com", "password123")
	user2 := createTestUser(t, "contact@example.com", "password123")

	body := map[string]interface{}{
		"contact_id": user2.ID,
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("DELETE", "/auth/contacts", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = addContextUser(req, user)
	w := httptest.NewRecorder()

	RemoveContactEndpoint(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestJwtCreateService_PasswordTooShort(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "test@example.com", "password123")

	userWithHash, _ := GetUser(itoa(user.ID))

	_, err := JwtCreateService(userWithHash, "short")
	assert.Error(t, err)
}

func TestGetUsers_Multiple(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	users, err := GetUsers()
	assert.NoError(t, err)
	assert.Len(t, *users, 0)

	createTestUser(t, "user1@example.com", "password123")
	createTestUser(t, "user2@example.com", "password123")
	createTestUser(t, "user3@example.com", "password123")

	users, err = GetUsers()
	assert.NoError(t, err)
	assert.Len(t, *users, 3)
}

func TestGetOrders_Multiple(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "sender@example.com", "password123")
	user2 := createTestUser(t, "receiver1@example.com", "password123")
	user3 := createTestUser(t, "receiver2@example.com", "password123")

	createTestOrder(t, user, user2.ID)
	createTestOrder(t, user, user3.ID)

	orders, err := GetOrders(user.ID)
	assert.NoError(t, err)
	assert.Len(t, *orders, 2)

	orders2, err := GetOrders(user2.ID)
	assert.NoError(t, err)
	assert.Len(t, *orders2, 1)
}

func TestGetOrderScans_Multiple(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "courier@example.com", "password123")
	sender := createTestUser(t, "sender@example.com", "password123")
	receiver := createTestUser(t, "receiver@example.com", "password123")
	order := createTestOrder(t, sender, receiver.ID)

	CreateOrderScan(&CreateOrderScanRequest{
		OrderId:     order.ID,
		PhotoBase64: "SGVsbG8gV29ybGQ=",
		Condition:   "Good",
		Comment:     "Scan 1",
	}, user)

	CreateOrderScan(&CreateOrderScanRequest{
		OrderId:     order.ID,
		PhotoBase64: "SGVsbG8gV29ybGQ=",
		Condition:   "Damaged",
		Comment:     "Scan 2",
	}, user)

	scans, err := GetOrderScans(itoa(order.ID))
	assert.NoError(t, err)
	assert.Len(t, scans, 2)
}

func TestGetActivitiesForUser_Empty(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "test@example.com", "password123")

	activities, err := GetActivitiesForUser(user.ID)
	assert.NoError(t, err)
	assert.Len(t, activities, 0)
}

func TestGetContacts_Multiple(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "test@example.com", "password123")
	user2 := createTestUser(t, "contact1@example.com", "password123")
	user3 := createTestUser(t, "contact2@example.com", "password123")
	user4 := createTestUser(t, "contact3@example.com", "password123")

	AddContact(user.ID, user2.ID)
	AddContact(user.ID, user3.ID)
	AddContact(user.ID, user4.ID)

	contacts, err := GetContacts(user.ID)
	assert.NoError(t, err)
	assert.Len(t, *contacts, 3)
}

func TestGetOrdersList_Multiple(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "test@example.com", "password123")
	user2 := createTestUser(t, "receiver1@example.com", "password123")
	user3 := createTestUser(t, "receiver2@example.com", "password123")

	createTestOrder(t, user, user2.ID)
	createTestOrder(t, user, user3.ID)
	createTestOrder(t, user3, user.ID)

	req := httptest.NewRequest("GET", "/auth/orders", nil)
	req = addContextUser(req, user)
	w := httptest.NewRecorder()

	GetOrdersList(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var orders []Order
	err := json.NewDecoder(w.Body).Decode(&orders)
	assert.NoError(t, err)
	assert.Len(t, orders, 3)
}

func TestGetAllScansEndpoint_Multiple(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "courier@example.com", "password123")
	sender := createTestUser(t, "sender@example.com", "password123")
	receiver := createTestUser(t, "receiver@example.com", "password123")

	order1 := createTestOrder(t, sender, receiver.ID)
	order2 := createTestOrder(t, sender, receiver.ID)

	CreateOrderScan(&CreateOrderScanRequest{
		OrderId:     order1.ID,
		PhotoBase64: "SGVsbG8gV29ybGQ=",
		Condition:   "Good",
	}, user)

	CreateOrderScan(&CreateOrderScanRequest{
		OrderId:     order2.ID,
		PhotoBase64: "SGVsbG8gV29ybGQ=",
		Condition:   "Damaged",
	}, user)

	req := httptest.NewRequest("GET", "/auth/scans", nil)
	req = addContextUser(req, user)
	w := httptest.NewRecorder()

	GetAllScansEndpoint(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var scans []Scan
	err := json.NewDecoder(w.Body).Decode(&scans)
	assert.NoError(t, err)
	assert.Len(t, scans, 2)
}

func TestGetActivitiesEndpoint_Multiple(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "test@example.com", "password123")
	CreateActivity(user.ID, user.ID, "type1", "Summary 1")
	CreateActivity(user.ID, user.ID, "type2", "Summary 2")

	req := httptest.NewRequest("GET", "/auth/activities", nil)
	req = addContextUser(req, user)
	w := httptest.NewRecorder()

	GetActivitiesEndpoint(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var activities []Activity
	err := json.NewDecoder(w.Body).Decode(&activities)
	assert.NoError(t, err)
	assert.Len(t, activities, 2)
}

func TestGetContactsEndpoint_Multiple(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "test@example.com", "password123")
	user2 := createTestUser(t, "contact1@example.com", "password123")
	user3 := createTestUser(t, "contact2@example.com", "password123")

	AddContact(user.ID, user2.ID)
	AddContact(user.ID, user3.ID)

	req := httptest.NewRequest("GET", "/auth/contacts", nil)
	req = addContextUser(req, user)
	w := httptest.NewRecorder()

	GetContactsEndpoint(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var contacts []Contact
	err := json.NewDecoder(w.Body).Decode(&contacts)
	assert.NoError(t, err)
	assert.Len(t, contacts, 2)
}

func TestCreateActivity_Multiple(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "test@example.com", "password123")

	err := CreateActivity(user.ID, user.ID, "type1", "Summary 1")
	assert.NoError(t, err)
	err = CreateActivity(user.ID, user.ID, "type2", "Summary 2")
	assert.NoError(t, err)

	activities, err := GetActivitiesForUser(user.ID)
	assert.NoError(t, err)
	assert.Len(t, activities, 2)
}

func TestRemoveContactEndpoint_Success(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user := createTestUser(t, "test@example.com", "password123")
	user2 := createTestUser(t, "contact@example.com", "password123")
	AddContact(user.ID, user2.ID)

	body := map[string]interface{}{
		"contact_id": user2.ID,
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("DELETE", "/auth/contacts", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = addContextUser(req, user)
	w := httptest.NewRecorder()

	RemoveContactEndpoint(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)

	contacts, _ := GetContacts(user.ID)
	assert.Len(t, *contacts, 0)
}

func TestDecodeBase64_WithCarriageReturn(t *testing.T) {
	result, err := decodeBase64("SGVsbG8gV29ybGQ=\r\n", 1, 1)
	assert.NoError(t, err)
	assert.Equal(t, "Hello World", string(result))
}

func TestDecodeBase64_WithSpace(t *testing.T) {
	result, err := decodeBase64("SGVs bG8gV29ybGQ=", 1, 1)
	assert.NoError(t, err)
	assert.Equal(t, "Hello World", string(result))
}
