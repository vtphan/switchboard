# Go Concurrency and Architecture Patterns - Practical LLM Guide

## Document Purpose
This document provides essential Go patterns for Large Language Model code generation, focusing on correctness, simplicity, and practical applicability. Complex patterns that rarely improve code quality have been excluded to enhance reasoning about code correctness.

## Core Concurrency Primitives

### Goroutines and Channels
```go
// Basic goroutine launch
go func() {
    // concurrent work
}()

// Unbuffered channel (synchronous)
ch := make(chan int)

// Buffered channel (asynchronous up to buffer size)
ch := make(chan int, 10)

// Send, receive, and close
ch <- 42           // send
value := <-ch      // receive
close(ch)

// Check if channel is closed
value, ok := <-ch
if !ok {
    // channel is closed
}
```

## Essential Concurrency Patterns

### 1. Pipeline Pattern
**Purpose**: Process data through sequential stages  
**Use Case**: Data transformation, ETL operations

```go
func pipeline() {
    // Stage 1: Generate numbers
    numbers := make(chan int)
    go func() {
        defer close(numbers)
        for i := 0; i < 10; i++ {
            numbers <- i
        }
    }()
    
    // Stage 2: Square numbers
    squares := make(chan int)
    go func() {
        defer close(squares)
        for n := range numbers {
            squares <- n * n
        }
    }()
    
    // Stage 3: Consume results
    for result := range squares {
        fmt.Println(result)
    }
}
```

### 2. Shared Channel Worker Pool Pattern
**Purpose**: Multiple workers sharing the same job channel for load balancing  
**Use Case**: Simple parallel processing without per-worker channels

```go
func sharedChannelWorkerPool(jobCount int, workerCount int) {
    jobs := make(chan int, jobCount)
    
    // Start workers - they all share the same jobs channel
    for i := 0; i < workerCount; i++ {
        go func(workerID int) {
            for job := range jobs {
                fmt.Printf("Worker %d processing job %d\n", workerID, job)
                processJob(job) // Do actual work
            }
        }(i)
    }
    
    // Send jobs
    for i := 0; i < jobCount; i++ {
        jobs <- i
    }
    close(jobs)
}
```

### 3. Fan-In Pattern
**Purpose**: Merge multiple input channels into single output  
**Use Case**: Aggregating results from multiple sources

```go
func fanIn(inputs ...<-chan int) <-chan int {
    output := make(chan int)
    var wg sync.WaitGroup
    
    // Start a goroutine for each input channel
    for _, input := range inputs {
        wg.Add(1)
        go func(ch <-chan int) {
            defer wg.Done()
            for value := range ch {
                output <- value
            }
        }(input)
    }
    
    // Close output when all inputs are done
    go func() {
        wg.Wait()
        close(output)
    }()
    
    return output
}
```

### 4. Fan-Out Pattern
**Purpose**: Distribute work from single input to multiple processors  
**Use Case**: Parallel processing, load distribution

```go
func fanOut(input <-chan int, workerCount int) []<-chan int {
    outputs := make([]<-chan int, workerCount)
    
    for i := 0; i < workerCount; i++ {
        output := make(chan int)
        outputs[i] = output
        
        go func(out chan<- int, workerID int) {
            defer close(out)
            for value := range input {
                // Only process if this worker should handle it
                if value%workerCount == workerID {
                    out <- value * value
                }
            }
        }(output, i)
    }
    
    return outputs
}
```

### 5. Bounded Parallelism with Semaphore Pattern
**Purpose**: Limit concurrent goroutines without worker pool overhead  
**Use Case**: Preventing runaway goroutine spawning

```go
func boundedParallelism(tasks []Task, maxConcurrent int) {
    sem := make(chan struct{}, maxConcurrent)
    var wg sync.WaitGroup
    
    for _, task := range tasks {
        wg.Add(1)
        sem <- struct{}{} // Acquire semaphore
        
        go func(t Task) {
            defer wg.Done()
            defer func() { <-sem }() // Release semaphore
            
            processTask(t)
        }(task)
    }
    
    wg.Wait()
}
```

## Select Statement Patterns

### 6. Timeout Pattern
```go
func withTimeout(ch <-chan string, timeout time.Duration) {
    select {
    case result := <-ch:
        fmt.Println("Received:", result)
    case <-time.After(timeout):
        fmt.Println("Timeout occurred")
    }
}
```

### 7. Non-blocking Operations
```go
func nonBlocking(ch chan int) {
    select {
    case ch <- 42:
        fmt.Println("Sent value")
    default:
        fmt.Println("Channel full, skipping")
    }
    
    select {
    case value := <-ch:
        fmt.Println("Received:", value)
    default:
        fmt.Println("No value available")
    }
}
```

### 8. Multiplexing
```go
func multiplex(ch1, ch2 <-chan string, done <-chan bool) {
    for {
        select {
        case msg1 := <-ch1:
            fmt.Println("From ch1:", msg1)
        case msg2 := <-ch2:
            fmt.Println("From ch2:", msg2)
        case <-done:
            fmt.Println("Shutting down")
            return
        }
    }
}
```

## Context Patterns

### 9. Context for Cancellation
```go
func cancellableOperation(ctx context.Context) error {
    for i := 0; i < 1000; i++ {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
            // Do work
            time.Sleep(10 * time.Millisecond)
        }
    }
    return nil
}

func useWithCancel() {
    ctx, cancel := context.WithCancel(context.Background())
    
    go func() {
        time.Sleep(100 * time.Millisecond)
        cancel() // Cancel after 100ms
    }()
    
    err := cancellableOperation(ctx)
    if err != nil {
        fmt.Println("Operation cancelled:", err)
    }
}
```

### 10. Context with Timeout
```go
func operationWithTimeout() {
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    defer cancel()
    
    result := make(chan string, 1)
    
    go func() {
        // Simulate work
        time.Sleep(3 * time.Second)
        result <- "completed"
    }()
    
    select {
    case res := <-result:
        fmt.Println("Result:", res)
    case <-ctx.Done():
        fmt.Println("Operation timed out")
    }
}
```

## Synchronization Patterns

### 11. WaitGroup Pattern
**Purpose**: Wait for multiple goroutines to complete

```go
func waitGroupExample() {
    var wg sync.WaitGroup
    
    for i := 0; i < 5; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            fmt.Printf("Worker %d completed\n", id)
            time.Sleep(time.Second)
        }(i)
    }
    
    wg.Wait()
    fmt.Println("All workers completed")
}
```

### 12. Mutex Patterns
```go
// Basic Mutex
type SafeCounter struct {
    mu    sync.Mutex
    count int
}

func (c *SafeCounter) Increment() {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.count++
}

func (c *SafeCounter) Value() int {
    c.mu.Lock()
    defer c.mu.Unlock()
    return c.count
}

// RWMutex for Read-Heavy Workloads
type SafeMap struct {
    mu   sync.RWMutex
    data map[string]int
}

func (sm *SafeMap) Get(key string) (int, bool) {
    sm.mu.RLock()
    defer sm.mu.RUnlock()
    value, exists := sm.data[key]
    return value, exists
}

func (sm *SafeMap) Set(key string, value int) {
    sm.mu.Lock()
    defer sm.mu.Unlock()
    sm.data[key] = value
}
```

### 13. Once Pattern
**Purpose**: Ensure initialization happens exactly once

```go
type Config struct {
    once sync.Once
    data map[string]string
}

func (c *Config) Load() {
    c.once.Do(func() {
        // Expensive initialization
        c.data = loadConfigFromFile()
    })
}
```

## Error Handling Patterns

### 14. Error Group Pattern
**Purpose**: Coordinate multiple goroutines with error handling

```go
import "golang.org/x/sync/errgroup"

func errorGroupExample() error {
    g, ctx := errgroup.WithContext(context.Background())
    
    urls := []string{"http://example.com", "http://google.com"}
    
    for _, url := range urls {
        url := url // capture loop variable
        g.Go(func() error {
            return fetchURL(ctx, url)
        })
    }
    
    return g.Wait() // Returns first error encountered
}
```

### 15. Error Channel Pattern
```go
type Result struct {
    Value string
    Error error
}

func processWithErrors(inputs []string) {
    results := make(chan Result, len(inputs))
    
    for _, input := range inputs {
        go func(in string) {
            value, err := processInput(in)
            results <- Result{Value: value, Error: err}
        }(input)
    }
    
    for i := 0; i < len(inputs); i++ {
        result := <-results
        if result.Error != nil {
            fmt.Printf("Error processing: %v\n", result.Error)
        } else {
            fmt.Printf("Result: %s\n", result.Value)
        }
    }
}
```

### 16. Recovery Wrapper Pattern
**Purpose**: Protect goroutines from panics

```go
func safeGoroutine(fn func()) {
    go func() {
        defer func() {
            if r := recover(); r != nil {
                log.Printf("Goroutine recovered from panic: %v", r)
            }
        }()
        fn()
    }()
}

// Usage example
func riskyOperations() {
    for i := 0; i < 5; i++ {
        safeGoroutine(func() {
            // Risky operation that might panic
            doRiskyWork()
        })
    }
}
```

## Rate Limiting Patterns

### 17. Ticker-Based Rate Limiting
**Purpose**: Strict interval-based rate limiting

```go
func rateLimiter(requests <-chan Request, rate time.Duration) <-chan Request {
    output := make(chan Request)
    ticker := time.NewTicker(rate)
    
    go func() {
        defer close(output)
        defer ticker.Stop()
        
        for req := range requests {
            <-ticker.C // Wait for rate limit
            output <- req
        }
    }()
    
    return output
}
```

### 18. Token Bucket Rate Limiting
**Purpose**: Allow bursts while enforcing average rate

```go
type TokenBucket struct {
    tokens     chan struct{}
    refillRate time.Duration
    capacity   int
}

func NewTokenBucket(capacity int, refillRate time.Duration) *TokenBucket {
    tb := &TokenBucket{
        tokens:     make(chan struct{}, capacity),
        refillRate: refillRate,
        capacity:   capacity,
    }
    
    // Pre-fill bucket
    for i := 0; i < capacity; i++ {
        tb.tokens <- struct{}{}
    }
    
    // Start refill process
    go tb.refill()
    
    return tb
}

func (tb *TokenBucket) refill() {
    ticker := time.NewTicker(tb.refillRate)
    defer ticker.Stop()
    
    for range ticker.C {
        select {
        case tb.tokens <- struct{}{}:
            // Token added
        default:
            // Bucket full, skip
        }
    }
}

func (tb *TokenBucket) Acquire() bool {
    select {
    case <-tb.tokens:
        return true
    default:
        return false // No tokens available
    }
}
```

## Testing Patterns

### 19. Context-Aware Testing
**Purpose**: Prevent hanging tests due to goroutine deadlocks

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

## Graceful Shutdown Pattern

### 20. Graceful Shutdown
**Purpose**: Clean shutdown of services with context

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
    
    log.Println("Server exited")
}
```

## Architecture Patterns

### 21. Controller-Service-Repository (CSR) Pattern
**Purpose**: Clean separation of responsibilities

```go
// Controller - handles HTTP
type UserHandler struct {
    service UserService
}

func (h *UserHandler) GetUser(w http.ResponseWriter, r *http.Request) {
    id := r.URL.Query().Get("id")
    user, err := h.service.GetUser(id)
    if err != nil {
        http.Error(w, "Not found", http.StatusNotFound)
        return
    }
    json.NewEncoder(w).Encode(user)
}

// Service - contains business logic
type UserService struct {
    repo UserRepository
}

func (s *UserService) GetUser(id string) (*User, error) {
    return s.repo.FindByID(id)
}

// Repository - manages data access
type UserRepository interface {
    FindByID(id string) (*User, error)
}
```

### 22. Functional Options Pattern
**Purpose**: Configurable constructors without bloated parameter lists

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
        addr: ":8080", // default
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

### 23. Middleware Chaining Pattern
**Purpose**: Layered processing for HTTP handlers

```go
func LoggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        next.ServeHTTP(w, r)
        log.Printf("%s %s %v", r.Method, r.URL.Path, time.Since(start))
    })
}

func AuthMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        token := r.Header.Get("Authorization")
        if token == "" {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }
        next.ServeHTTP(w, r)
    })
}

// Usage
http.Handle("/api/", LoggingMiddleware(AuthMiddleware(apiHandler)))
```

### 24. Interface-Driven Dependency Injection
**Purpose**: Decouple dependencies through interfaces

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
    // Business logic
    processed := strings.ToUpper(item)
    return s.store.Save(processed)
}

// Implementation
type FileStore struct {
    basePath string
}

func (fs *FileStore) Save(item string) error {
    // Save to file
    return nil
}

func (fs *FileStore) Get(id string) (string, error) {
    // Read from file
    return "", nil
}
```

### 25. Retry with Backoff Pattern
**Purpose**: Resilient operations with exponential backoff

```go
func retryWithBackoff(attempts int, initialDelay time.Duration, fn func() error) error {
    var err error
    delay := initialDelay
    
    for i := 0; i < attempts; i++ {
        err = fn()
        if err == nil {
            return nil
        }
        
        if i < attempts-1 { // Don't sleep on last attempt
            time.Sleep(delay)
            delay *= 2 // Exponential backoff
        }
    }
    
    return fmt.Errorf("all %d attempts failed, last error: %v", attempts, err)
}

// Usage
err := retryWithBackoff(3, 100*time.Millisecond, func() error {
    return makeHTTPRequest()
})
```

## Pattern Selection Guide

| Use Case | Recommended Pattern | Alternative |
|----------|-------------------|-------------|
| Data processing pipeline | Pipeline Pattern | Fan-Out + Fan-In |
| Limited concurrent operations | Bounded Parallelism | Shared Channel Worker Pool |
| Simple load balancing | Shared Channel Worker Pool | Fan-Out |
| Merging multiple sources | Fan-In | Select multiplexing |
| Timed operations | Select with Timeout | Context with Timeout |
| Burst-friendly rate limiting | Token Bucket | Ticker-based Rate Limiting |
| Strict interval rate limiting | Ticker-based Rate Limiting | Token Bucket with small capacity |
| Shared state protection | Mutex/RWMutex | Channel-based coordination |
| One-time initialization | sync.Once | Context-based initialization |
| Multiple goroutine coordination | WaitGroup + Error Group | Channel coordination |
| Clean service shutdown | Graceful Shutdown | Context cancellation |
| External API resilience | Retry with Backoff + Rate Limiting | Circuit Breaker |
| Test safety | Context-aware Testing | Timeout-based helpers |
| HTTP service structure | CSR + Middleware + Graceful Shutdown | Simple handler functions |

## Common Pitfalls to Avoid

### 1. Goroutine Leaks
```go
// BAD: Goroutine may never exit
go func() {
    for {
        select {
        case data := <-ch:
            process(data)
        }
    }
}()

// GOOD: Provide exit condition
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
```

### 2. Channel Deadlocks
```go
// BAD: Will deadlock
ch := make(chan int)
ch <- 1 // No reader available

// GOOD: Use buffered channel or separate goroutine
ch := make(chan int, 1)
ch <- 1
```

### 3. Race Conditions
```go
// BAD: Race condition
var counter int
go func() { counter++ }()
go func() { counter++ }()

// GOOD: Use atomic operations or mutex
var counter int64
go func() { atomic.AddInt64(&counter, 1) }()
go func() { atomic.AddInt64(&counter, 1) }()
```

### 4. Missing Context Cancellation
```go
// BAD: No way to cancel long-running operation
func longRunningTask() {
    for i := 0; i < 1000000; i++ {
        doWork()
    }
}

// GOOD: Check context cancellation
func longRunningTask(ctx context.Context) error {
    for i := 0; i < 1000000; i++ {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
            doWork()
        }
    }
    return nil
}
```

## Best Practices for LLM Code Generation

### Always Do:
1. **Close channels** when done sending
2. **Use defer** for cleanup operations (unlock, close, etc.)
3. **Handle context cancellation** in long-running operations
4. **Check for channel closure** when receiving: `value, ok := <-ch`
5. **Use buffered channels** to prevent blocking when appropriate
6. **Implement timeouts** for external operations
7. **Wrap risky goroutines** with panic recovery
8. **Test concurrent code** with timeouts to prevent hanging tests

### Pattern Priorities:
1. **Essential**: Context cancellation, bounded parallelism, proper cleanup
2. **Important**: Error handling, testing patterns, graceful shutdown
3. **Useful**: Rate limiting, CSR architecture, functional options
4. **Advanced**: Only when simple patterns prove insufficient

### Decision Framework:
1. **Start with the simplest pattern** that solves the problem
2. **Measure performance** before optimizing
3. **Prefer channels for communication**, mutexes for protecting state
4. **Use context** for cancellation and timeouts
5. **Test concurrency** with proper timeout mechanisms

This guide focuses on patterns that improve code correctness and maintainability while avoiding over-engineering. The Go runtime is already highly optimized - focus on writing clear, correct concurrent code rather than premature optimization.