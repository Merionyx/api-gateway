#!/bin/bash

# Простой скрипт для тестирования API

echo "🧪 Тестирование API Gateway Control Plane"
echo "========================================="

BASE_URL="http://localhost:8080"

echo ""
echo "1. Проверка health check..."
curl -s "$BASE_URL/health" | jq '.' || echo "Health check failed"

echo ""
echo "2. Получение списка тенантов..."
curl -s "$BASE_URL/api/v1/tenants" | jq '.' || echo "Get tenants failed"

echo ""
echo "3. Создание тенанта..."
TENANT_RESPONSE=$(curl -s -X POST "$BASE_URL/api/v1/tenants" \
  -H "Content-Type: application/json" \
  -d '{"name": "test-tenant"}')

echo "$TENANT_RESPONSE" | jq '.' || echo "Create tenant failed"

# Извлекаем ID тенанта для дальнейших тестов
TENANT_ID=$(echo "$TENANT_RESPONSE" | jq -r '.id' 2>/dev/null)

if [ "$TENANT_ID" != "null" ] && [ -n "$TENANT_ID" ]; then
    echo ""
    echo "4. Получение тенанта по ID: $TENANT_ID"
    curl -s "$BASE_URL/api/v1/tenants/$TENANT_ID" | jq '.' || echo "Get tenant by ID failed"
    
    echo ""
    echo "5. Создание окружения для тенанта..."
    ENV_RESPONSE=$(curl -s -X POST "$BASE_URL/api/v1/environments" \
      -H "Content-Type: application/json" \
      -d "{\"name\": \"test-env\", \"tenant_id\": \"$TENANT_ID\", \"config\": {\"port\": 8080}}")
    
    echo "$ENV_RESPONSE" | jq '.' || echo "Create environment failed"
    
    ENV_ID=$(echo "$ENV_RESPONSE" | jq -r '.id' 2>/dev/null)
    
    if [ "$ENV_ID" != "null" ] && [ -n "$ENV_ID" ]; then
        echo ""
        echo "6. Создание слушателя для окружения..."
        LISTENER_RESPONSE=$(curl -s -X POST "$BASE_URL/api/v1/listeners" \
          -H "Content-Type: application/json" \
          -d "{\"name\": \"test-listener\", \"environment_id\": \"$ENV_ID\", \"config\": {\"address\": \"0.0.0.0:8080\"}}")
        
        echo "$LISTENER_RESPONSE" | jq '.' || echo "Create listener failed"
    fi
fi

echo ""
echo "✅ Тестирование завершено!"
