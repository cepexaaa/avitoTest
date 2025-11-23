#!/bin/bash

echo "E2E Testing PR Review Service"

BASE_URL="http://localhost:8080"
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

PASS_COUNT=0
FAIL_COUNT=0

echo "=========================================="

test_endpoint() {
    local description=$1
    local expected_status=$2
    local url=$3
    local data=$4
    local method=${5:-"POST"}
    
    local response=$(curl -s -o response.json -w "%{http_code}" -X "$method" "$url" -H "Content-Type: application/json" -d "$data")
    
    if [ "$response" -eq "$expected_status" ]; then
        echo -e "${GREEN}✓ PASS${NC}: $description"
        ((PASS_COUNT++))
        # Показываем ответ только для успешных запросов
        if [ "$expected_status" -eq 200 ] || [ "$expected_status" -eq 201 ]; then
            cat response.json | jq . 2>/dev/null || cat response.json
        fi
    else
        echo -e "${RED}✗ FAIL${NC}: $description"
        echo -e "  Expected: $expected_status, Got: $response"
        ((FAIL_COUNT++))
        # Для ошибок показываем тело ответа
        if [ -f "response.json" ]; then
            echo -e "  Response:"
            cat response.json | jq . 2>/dev/null || cat response.json
        fi
    fi
    echo
}

cleanup() {
    rm -f response.json
}

# Очистка перед началом
cleanup

echo "=== 1. Positive Test Cases ==="

# 1.1 Создание команды
echo "1.1 Creating team..."
test_endpoint "Create team 'developers'" 201 "$BASE_URL/team/add" '{
    "team_name": "developers",
    "members": [
      {"user_id": "u1", "username": "Alice", "is_active": true},
      {"user_id": "u2", "username": "Bob", "is_active": true},
      {"user_id": "u3", "username": "Charlie", "is_active": true}
    ]
}'

# 1.2 Создание второй команды
echo "1.2 Creating second team..."
test_endpoint "Create team 'backend'" 201 "$BASE_URL/team/add" '{
    "team_name": "backend",
    "members": [
      {"user_id": "u4", "username": "David", "is_active": true},
      {"user_id": "u5", "username": "Eve", "is_active": true}
    ]
}'

# 1.3 Получение команды
echo "1.3 Getting team..."
test_endpoint "Get team 'developers'" 200 "$BASE_URL/team/get?team_name=developers" "" "GET"

# 1.4 Создание PR с авто-назначением ревьюеров
echo "1.4 Creating PR with auto-assignment..."
test_endpoint "Create PR with author from team" 201 "$BASE_URL/pullRequest/create" '{
    "pull_request_id": "pr-001",
    "pull_request_name": "Add authentication",
    "author_id": "u1"
}'

# 1.5 Проверка что назначились 2 ревьюера
echo "1.5 Checking reviewers assignment..."
response=$(curl -s -X POST "$BASE_URL/pullRequest/create" -H "Content-Type: application/json" -d '{
    "pull_request_id": "pr-business-test",
    "pull_request_name": "Business Logic Test",
    "author_id": "u1"
}')
reviewers_count=$(echo "$response" | jq -r '.pr.assigned_reviewers | length' 2>/dev/null)
if [ "$reviewers_count" -eq 2 ]; then
    echo -e "${GREEN}✓ PASS${NC}: Auto-assigned exactly 2 reviewers"
    ((PASS_COUNT++))
else
    echo -e "${RED}✗ FAIL${NC}: Expected 2 reviewers, got $reviewers_count"
    ((FAIL_COUNT++))
fi
echo

# 1.6 Получение PR пользователя
echo "1.6 Getting user PRs..."
test_endpoint "Get PRs assigned to user u2" 200 "$BASE_URL/users/getReview?user_id=u2" "" "GET"

# 1.7 Мерж PR
echo "1.7 Merging PR..."
test_endpoint "Merge PR pr-001" 200 "$BASE_URL/pullRequest/merge" '{
    "pull_request_id": "pr-001"
}'

# 1.8 Идемпотентность мержа
echo "1.8 Testing merge idempotency..."
test_endpoint "Merge same PR again (idempotent)" 200 "$BASE_URL/pullRequest/merge" '{
    "pull_request_id": "pr-001"
}'

# 1.9 Переназначение ревьюера
echo "1.9 Reassigning reviewer..."
test_endpoint "Reassign reviewer in open PR" 200 "$BASE_URL/pullRequest/reassign" '{
    "pull_request_id": "pr-business-test",
    "old_user_id": "u2"
}'

# 1.10 Деактивация пользователя
echo "1.10 Deactivating user..."
test_endpoint "Deactivate user u3" 200 "$BASE_URL/users/setIsActive" '{
    "user_id": "u3",
    "is_active": false
}'

echo "=== 2. Error Test Cases ==="

# 2.1 Создание команды с существующим именем
echo "2.1 Creating duplicate team..."
test_endpoint "Create team with duplicate name" 400 "$BASE_URL/team/add" '{
    "team_name": "developers",
    "members": [
      {"user_id": "u10", "username": "John", "is_active": true}
    ]
}'

# 2.2 Получение несуществующей команды
echo "2.2 Getting non-existent team..."
test_endpoint "Get non-existent team" 404 "$BASE_URL/team/get?team_name=nonexistent" "" "GET"

# 2.3 Создание PR для несуществующего пользователя
echo "2.3 Creating PR for non-existent user..."
test_endpoint "Create PR with non-existent author" 404 "$BASE_URL/pullRequest/create" '{
    "pull_request_id": "pr-002",
    "pull_request_name": "Invalid PR",
    "author_id": "nonexistent-user"
}'

echo "2.4 Reassigning in merged PR..."
test_endpoint "Reassign reviewer in merged PR" 409 "$BASE_URL/pullRequest/reassign" '{
    "pull_request_id": "pr-001",
    "old_user_id": "u2"
}'

# 2.4 Создание PR с существующим ID
echo "2.5 Creating PR with duplicate ID..."
test_endpoint "Create PR with duplicate ID" 201 "$BASE_URL/pullRequest/create" '{
    "pull_request_id": "pr-001",
    "pull_request_name": "Duplicate PR",
    "author_id": "u1"
}'

# 2.5 Мерж несуществующего PR
echo "2.6 Merging non-existent PR..."
test_endpoint "Merge non-existent PR" 404 "$BASE_URL/pullRequest/merge" '{
    "pull_request_id": "pr-nonexistent"
}'

# 2.7 Переназначение не назначенного ревьюера
echo "2.7 Reassigning non-assigned reviewer..."
test_endpoint "Reassign non-assigned reviewer" 409 "$BASE_URL/pullRequest/reassign" '{
    "pull_request_id": "pr-business-test",
    "old_user_id": "u5" 
}'

# 2.8 Получение PR для несуществующего пользователя
echo "2.8 Getting PRs for non-existent user..."
test_endpoint "Get PRs for non-existent user" 200 "$BASE_URL/users/getReview?user_id=nonexistent" "" "GET"

# 2.9 Деактивация несуществующего пользователя
echo "2.9 Deactivating non-existent user..."
test_endpoint "Deactivate non-existent user" 404 "$BASE_URL/users/setIsActive" '{
    "user_id": "nonexistent",
    "is_active": false
}'

echo "=== 3. Edge Cases ==="

# 3.1 Команда с одним пользователем
echo "3.1 Team with single user..."
test_endpoint "Create team with single user" 201 "$BASE_URL/team/add" '{
    "team_name": "solo-team",
    "members": [
      {"user_id": "solo1", "username": "SoloUser", "is_active": true}
    ]
}'

# 3.2 PR в команде с одним пользователем (должен быть 0 ревьюеров)
echo "3.2 PR in single-user team..."
response=$(curl -s -X POST "$BASE_URL/pullRequest/create" -H "Content-Type: application/json" -d '{
    "pull_request_id": "pr-solo",
    "pull_request_name": "Solo PR",
    "author_id": "solo1"
}')
solo_reviewers_count=$(echo "$response" | jq -r '.pr.assigned_reviewers | length' 2>/dev/null)
if [ "$solo_reviewers_count" -eq 0 ]; then
    echo -e "${GREEN}✓ PASS${NC}: No reviewers assigned in single-user team"
    ((PASS_COUNT++))
else
    echo -e "${RED}✗ FAIL${NC}: Expected 0 reviewers in single-user team, got $solo_reviewers_count"
    ((FAIL_COUNT++))
fi
echo

# 3.3 Деактивированный пользователь не должен назначаться
echo "3.3 Deactivated user should not be assigned..."
# Создаем PR после деактивации u3
response=$(curl -s -X POST "$BASE_URL/pullRequest/create" -H "Content-Type: application/json" -d '{
    "pull_request_id": "pr-after-deactivate",
    "pull_request_name": "Test after deactivate",
    "author_id": "u1"
}')
deactivated_assigned=$(echo "$response" | jq -r '.pr.assigned_reviewers | contains(["u3"])' 2>/dev/null)
if [ "$deactivated_assigned" = "false" ]; then
    echo -e "${GREEN}✓ PASS${NC}: Deactivated user u3 not assigned as reviewer"
    ((PASS_COUNT++))
else
    echo -e "${RED}✗ FAIL${NC}: Deactivated user u3 was incorrectly assigned"
    ((FAIL_COUNT++))
fi
echo

# Очистка
cleanup

echo "=========================================="
echo -e "${YELLOW}TEST SUMMARY:${NC}"
echo -e "${GREEN}Passed: $PASS_COUNT${NC}"
echo -e "${RED}Failed: $FAIL_COUNT${NC}"

if [ "$FAIL_COUNT" -eq 0 ]; then
    echo -e "${GREEN}🎉 All tests passed!${NC}"
    exit 0
else
    echo -e "${RED}❌ Some tests failed!${NC}"
    exit 1
fi