# Probo v1 - Prediction Market Platform

## Architecture

<img width="1679" height="1191" alt="proBooArch" src="https://github.com/user-attachments/assets/6e054b2f-4cf0-43fc-8cc0-b1175374ad9c" />

## Demo Video

[![Prediction Platform demo video](https://img.youtube.com/vi/zRrTNtnqacE/0.jpg)](https://youtu.be/zRrTNtnqacE)

### Services

- **Database**: In-memory data persistence for orders, users, balances, and markets
- **Engine**: Core matching engine with order book management and balance calculations
- **Server**: REST API server handling HTTP requests and responses
- **Socket**: WebSocket server for real-time client updates
- **Shared**: Common types and Redis client utilities
- **Test**: Integration testing suite

## Usage

### Creating a User

```bash
curl -X POST http://localhost:8080/user/testuser
```

### Creating a Prediction Market

```bash
curl -X POST http://localhost:8080/symbol/createmarket \
  -H "Content-Type: application/json" \
  -d '{
    "symbol": "BTC_PREDICT",
    "marketType": "manual",
    "endsIn": 30000,
    "sourceOfTruth": "manual",
    "endAfterTime": 30000,
    "heading": "Will BTC be above $100k?",
    "eventType": "crypto"
  }'
```

### Placing Orders

```bash
# Buy YES shares
curl -X POST http://localhost:8080/order/buy \
  -H "Content-Type: application/json" \
  -d '{
    "userId": "testuser",
    "stockSymbol": "BTC_PREDICT",
    "quantity": 1,
    "price": 60,
    "stockType": "yes"
  }'

# Sell NO shares
curl -X POST http://localhost:8080/order/sell \
  -H "Content-Type: application/json" \
  -d '{
    "userId": "testuser",
    "stockSymbol": "BTC_PREDICT",
    "quantity": 1,
    "price": 40,
    "stockType": "no"
  }'
```

### Checking Balances

```bash
# Get USD balance
curl http://localhost:8080/balance/get/testuser

# Get stock positions
curl http://localhost:8080/balance/stocks/testuser
```

### WebSocket Real-time Updates

```javascript
const ws = new WebSocket("ws://localhost:8081/ws");

// Subscribe to market updates
ws.onopen = () => {
  ws.send(
    JSON.stringify({
      type: "subscribe",
      symbol: "BTC_PREDICT",
    })
  );
};

ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  console.log("Order book update:", data);
};
```

### Ending a Market

```bash
curl -X POST http://localhost:8080/order/endmarket \
  -H "Content-Type: application/json" \
  -d '{
    "stockSymbol": "BTC_PREDICT",
    "marketId": "BTC_PREDICT",
    "winningStock": "yes"
  }'
```

## API Reference

### User Management

- `POST /user/:id` - Create new user
- `GET /balance/get/:userId` - Get USD balance
- `GET /balance/stocks/:userId` - Get stock positions

### Order Management

- `POST /order/buy` - Place buy order
- `POST /order/sell` - Place sell order
- `POST /order/cancel` - Cancel order

### Market Management

- `POST /symbol/createmarket` - Create prediction market
- `POST /order/endmarket` - End market and settle

### Order Book

- `GET /book/get` - Get all order books
- `GET /book/get/:symbol` - Get specific order book

## Testing

Run integration tests:

```bash
task test
```

The test suite creates users, places orders, checks balances, and validates market settlement.

## Data Flow

1. **Order Placement**: HTTP request → Redis queue → Engine validation → Database storage
2. **Order Matching**: Engine checks order book → Matches if possible → Updates balances
3. **Real-time Updates**: Engine publishes changes → WebSocket broadcasts to clients
4. **Market Settlement**: Admin triggers end → Engine processes payouts → Database updates
