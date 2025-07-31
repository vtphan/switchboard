# Go Patterns Reference for Claude Code

## Quick Decision Guide

**Pattern Selection Priority:**
1. **Essential Core** → Use for 80% of cases
2. **Common Scenarios** → Use for specific needs  
3. **Advanced Patterns** → Use only when simpler patterns insufficient

---

# ESSENTIAL CORE PATTERNS (Start Here)

## 1. Goroutines & Channels
```go
// Basic concurrent execution
go func() { /* work */ }()

// Channel communication
ch := make(chan int, 10)  // buffered
ch <- value               // send
result := <-ch           // receive
close(ch)                // close when done sending

// Check if closed
value, ok := <-ch
if !ok { /* channel closed */ }
```

## 2. Context for Cancellation
```go
func operation(ctx context.Context) error {
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()  // Handle cancellation
        default:
            // Do work
        }
    }
}

// Usage
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
```

## 3. Bounded Parallelism (Semaphore)
```go
func boundedWork(tasks []Task, maxConcurrent int) {
    sem := make(chan struct{}, maxConcurrent)
    var wg sync.WaitGroup
    
    for _, task := range tasks {
        wg.Add(1)
        sem <- struct{}{}  // Acquire
        
        go func(t Task) {
            defer wg.Done()
            defer func() { <-sem }()  // Release
            processTask(t)
        }(task)
    }
    wg.Wait()
}
```

## 4. Error Handling with errgroup
```go
import "golang.org/x/sync/errgroup"

func processAll(items []string) error {
    g, ctx := errgroup.WithContext(context.Background())
    
    for _, item := range items {
        item := item  // Capture loop variable
        g.Go(func() error {
            return processItem(ctx, item)
        })
    }
    
    return g.Wait()  // Returns first error or nil
}
```

## 5. Safe Goroutine Recovery
```go
func safeGoroutine(fn func()) {
    go func() {
        defer func() {
            if r := recover(); r != nil {
                log.Printf("Recovered panic: %v", r)
            }
        }()
        fn()
    }()
}
```

---

# COMMON SCENARIO PATTERNS

## Pipeline Processing
```go
func pipeline() {
    // Stage 1: Generate
    numbers := make(chan int)
    go func() {
        defer close(numbers)
        for i := 0; i < 10; i++ {
            numbers <- i
        }
    }()
    
    // Stage 2: Transform
    squares := make(chan int)
    go func() {
        defer close(squares)
        for n := range numbers {
            squares <- n * n
        }
    }()
    
    // Stage 3: Consume
    for result := range squares {
        fmt.Println(result)
    }
}
```

## Shared Worker Pool
```go
func workerPool(jobs <-chan int, workerCount int) {
    for i := 0; i < workerCount; i++ {
        go func(id int) {
            for job := range jobs {
                fmt.Printf("Worker %d processing %d\n", id, job)
                processJob(job)
            }
        }(i)
    }
}
```

## Fan-In (Merge Channels)
```go
func fanIn(inputs ...<-chan int) <-chan int {
    output := make(chan int)
    var wg sync.WaitGroup
    
    for _, input := range inputs {
        wg.Add(1)
        go func(ch <-chan int) {
            defer wg.Done()
            for value := range ch {
                output <- value
            }
        }(input)
    }
    
    go func() {
        wg.Wait()
        close(output)
    }()
    
    return output
}
```

## Rate Limiting
```go
// Token Bucket (allows bursts)
type TokenBucket struct {
    tokens chan struct{}
}

func NewTokenBucket(capacity int, refillRate time.Duration) *TokenBucket {
    tb := &TokenBucket{tokens: make(chan struct{}, capacity)}
    
    // Pre-fill bucket
    for i := 0; i < capacity; i++ {
        tb.tokens <- struct{}{}
    }
    
    // Start refill process
    go func() {
        ticker := time.NewTicker(refillRate)
        defer ticker.Stop()
        for range ticker.C {
            select {
            case tb.tokens <- struct{}{}:
            default: // Bucket full
            }
        }
    }()
    
    return tb
}

func (tb *TokenBucket) Acquire() bool {
    select {
    case <-tb.tokens:
        return true
    default:
        return false
    }
}

// Ticker-based (strict intervals)
func rateLimiter(requests <-chan Request, rate time.Duration) <-chan Request {
    output := make(chan Request)
    ticker := time.NewTicker(rate)
    
    go func() {
        defer close(output)
        defer ticker.Stop()
        for req := range requests {
            <-ticker.C
            output <- req
        }
    }()
    
    return output
}
```

## Timeout & Select Patterns
```go
// Basic timeout
select {
case result := <-ch:
    // Got result
case <-time.After(5 * time.Second):
    // Timeout
}

// Non-blocking operations
select {
case ch <- value:
    // Sent
default:
    // Channel full
}

// Multiplexing
select {
case msg1 := <-ch1:
    // Handle ch1
case msg2 := <-ch2:
    // Handle ch2
case <-done:
    return
}
```

---

# ARCHITECTURAL PATTERNS

## Controller-Service-Repository (CSR)
```go
// Controller - HTTP layer
type UserHandler struct {
    service UserService
}

func (h *UserHandler) GetUser(w http.ResponseWriter, r *http.Request) {
    id := r.URL.Query().Get("id")
    user, err := h.service.GetUser(id)
    if err != nil {
        http.Error(w, "Not found", 404)
        return
    }
    json.NewEncoder(w).Encode(user)
}

// Service - Business logic
type UserService struct {
    repo UserRepository
}

func (s *UserService) GetUser(id string) (*User, error) {
    return s.repo.FindByID(id)
}

// Repository - Data access
type UserRepository interface {
    FindByID(id string) (*User, error)
}
```

## Functional Options
```go
type Server struct {
    addr string
    log  *log.Logger
}

type Option func(*Server)

func WithAddr(addr string) Option {
    return func(s *Server) { s.addr = addr }
}

func WithLogger(l *log.Logger) Option {
    return func(s *Server) { s.log = l }
}

func NewServer(opts ...Option) *Server {
    s := &Server{
        addr: ":8080",
        log:  log.New(os.Stdout, "", log.LstdFlags),
    }
    for _, opt := range opts {
        opt(s)
    }
    return s
}

// Usage
server := NewServer(
    WithAddr(":9000"),
    WithLogger(customLogger),
)
```

## Interface-Driven Dependency Injection
```go
type Store interface {
    Save(item string) error
    Get(id string) (string, error)
}

type Service struct {
    store Store
}

func NewService(s Store) *Service {
    return &Service{store: s}
}

func (s *Service) ProcessItem(item string) error {
    processed := strings.ToUpper(item)
    return s.store.Save(processed)
}
```

## Middleware Pattern
```go
func LoggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        next.ServeHTTP(w, r)
        log.Printf("%s %s %v", r.Method, r.URL.Path, time.Since(start))
    })
}

// Chain middlewares
http.Handle("/api/", LoggingMiddleware(AuthMiddleware(apiHandler)))
```

---

# TESTING & RESILIENCE PATTERNS

## Context-Aware Testing
```go
func TestConcurrentFunction(t *testing.T) {
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    defer cancel()
    
    done := make(chan struct{})
    var result string
    
    go func() {
        defer close(done)
        result = myConcurrentFunc()
    }()
    
    select {
    case <-done:
        if result != "expected" {
            t.Errorf("got %s, want expected", result)
        }
    case <-ctx.Done():
        t.Fatal("test timed out - possible deadlock")
    }
}
```

## Retry with Backoff
```go
func retryWithBackoff(attempts int, initialDelay time.Duration, fn func() error) error {
    var err error
    delay := initialDelay
    
    for i := 0; i < attempts; i++ {
        err = fn()
        if err == nil {
            return nil
        }
        
        if i < attempts-1 {
            time.Sleep(delay)
            delay *= 2  // Exponential backoff
        }
    }
    
    return fmt.Errorf("all %d attempts failed, last error: %v", attempts, err)
}
```

## Graceful Shutdown
```go
func gracefulShutdown(server *http.Server) {
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

    <-quit
    log.Println("Shutting down server...")
    
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    if err := server.Shutdown(ctx); err != nil {
        log.Fatalf("Server forced to shutdown: %v", err)
    }
}
```

---

# ADVANCED PATTERNS (Use Sparingly)

## Channel of Channels (Dynamic Routing)
```go
type TaskChannel chan int

func channelOfChannels() {
    master := make(chan TaskChannel)
    
    go func() {
        defer close(master)
        for task := range master {
            go func(t TaskChannel) {
                result := compute()
                t <- result
                close(t)
            }(task)
        }
    }()
    
    // Usage
    taskChan := make(TaskChannel, 1)
    master <- taskChan
    result := <-taskChan
}
```

## State Machine with Channels
```go
type State int
type Event int

const (
    StateIdle State = iota
    StateRunning
    StateStopped
)

type StateMachine struct {
    state  State
    events chan Event
}

func (sm *StateMachine) run() {
    for event := range sm.events {
        switch sm.state {
        case StateIdle:
            if event == EventStart {
                sm.state = StateRunning
            }
        case StateRunning:
            if event == EventStop {
                sm.state = StateStopped
            }
        }
    }
}
```

---

# DECISION FRAMEWORK

## Pattern Selection Guide

| Use Case | First Choice | Alternative |
|----------|-------------|-------------|
| **Concurrent tasks** | Bounded Parallelism | Worker Pool |
| **Pipeline processing** | Pipeline Pattern | Fan-Out + Fan-In |
| **Merge channels** | Fan-In | Select multiplexing |
| **Rate limiting (bursts OK)** | Token Bucket | Ticker-based |
| **Rate limiting (strict)** | Ticker-based | Token Bucket (small) |
| **Error coordination** | errgroup | Error channels |
| **Timeouts** | Context with timeout | Select with timer |
| **Shared state** | Mutex | Channel coordination |
| **HTTP services** | CSR + Middleware | Simple handlers |
| **Configuration** | Functional Options | Struct literals |
| **Testing concurrency** | Context timeouts | Done channels |

## Anti-Patterns to Avoid

```go
// ❌ Goroutine leaks (no exit condition)
go func() {
    for {
        select {
        case data := <-ch:
            process(data)
        }
    }
}()

// ✅ Proper cleanup
go func() {
    for {
        select {
        case data := <-ch:
            process(data)
        case <-done:
            return
        }
    }
}()

// ❌ Deadlock (no reader)
ch := make(chan int)
ch <- 1

// ✅ Buffered or separate goroutine
ch := make(chan int, 1)
ch <- 1

// ❌ Race condition
var counter int
go func() { counter++ }()

// ✅ Atomic or mutex
var counter int64
go func() { atomic.AddInt64(&counter, 1) }()
```

## Best Practices Checklist

- [ ] **Always** provide exit conditions for goroutines
- [ ] **Always** close channels when done sending
- [ ] **Always** handle context cancellation in long operations
- [ ] **Always** use defer for cleanup (unlock, close, etc.)
- [ ] **Test** concurrent code with timeouts
- [ ] **Check** for channel closure: `value, ok := <-ch`
- [ ] **Wrap** risky goroutines with panic recovery
- [ ] **Use** buffered channels to prevent blocking when appropriate

---

This reference prioritizes **correctness over performance** and **simplicity over cleverness**. Start with Essential Core patterns and only move to Advanced patterns when simpler solutions prove insufficient.