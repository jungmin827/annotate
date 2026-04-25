# ROADMAP.md — 개발 로드맵

## 전체 방향

```
Phase 1  Python으로 로직 검증
Phase 2  Go로 서버 전환 + 대시보드
Phase 3  자동화 + DB 마이그레이션
Phase 4  분석 파이프라인 고도화
```

데이터 구조와 분석 로직을 먼저 Python으로 빠르게 검증한다.
구조가 확정되면 Go로 옮긴다. Go로 바로 시작하지 않는다.

---

## Phase 1 — Python 프로토타입

**목표**: 로직이 실제로 동작하는지 검증한다.

### 스택
```
Python + yfinance + pandas-ta + anthropic + streamlit
```

### 구현 항목

- [ ] `scripts/fetch_market.py` — 주가 OHLCV 수집 + 지표 계산
- [ ] `scripts/analyze.py` — 단건 매매 분석 (Claude API 호출)
- [ ] `scripts/review.py` — 전체 이력 패턴 분석
- [ ] `scripts/brief.py` — 보유 종목 뉴스 브리핑
- [ ] `dashboard/app.py` — Streamlit 대시보드 (보유 현황, 수익률, 차트)
- [ ] `data/trades.json` — 실제 매매 데이터 5건 이상 등록

### 완료 기준
`/stock-ops analyze {id}` 실행 시 의미 있는 피드백이 나오는가.
패턴 문장이 실제 내 매매 습관을 반영하는가.

---

## Phase 2 — Go 서버 전환 ✅ 완료

**목표**: Python 스크립트를 Go HTTP 서버로 전환한다. 항상 켜져 있는 대시보드.

### 스택
```
Go + HTMX + SQLite (modernc.org/sqlite)
```

### 구현 내용 (커밋: 23b08a4)

- [x] `go/internal/store/db.go` — SQLite DB 오픈 + 스키마 적용 (`trades`, `market_cache` 테이블)
- [x] `go/internal/store/trades.go` — Trade CRUD (`InsertTrade`, `GetTradeByID`, `GetAllTrades`, `GetHoldingTrades`)
- [x] `go/internal/market/fetcher.go` — Yahoo Finance OHLCV 수집 (baseURL 주입으로 테스트 격리)
- [x] `go/internal/market/indicators.go` — RSI14, EMA, 거래량 비율 계산 + `SummarizeOnDate`
- [x] `go/internal/analysis/engine.go` — Claude API 호출 (`CallClaude`), `BuildAnalysisPrompt`, `BuildReviewPrompt`
- [x] `go/internal/analysis/pattern.go` — `ComputeStats` (종목별 PnL%, 보유일수 집계)
- [x] `go/internal/handler/handler.go` — HTTP 핸들러: `GET /`, `GET /trades`, `POST /analyze/{id}`, `POST /review`
- [x] `go/internal/handler/templates/` — HTMX 대시보드 (`dashboard.html`, `trades.html`, `analyze_result.html`, `review_result.html`)
- [x] `go/cmd/server/main.go` — 서버 진입점 (포트 8080)

### 실제 완료 기준
- 전체 테스트 12개 PASS
- 브라우저에서 대시보드 접근, 매매 분석(HTMX) 동작 확인

---

## Phase 3 — 뉴스 크롤러 + 백그라운드 스케줄러 ✅ 완료

**목표**: 보유 종목 뉴스를 자동 수집하고 Claude로 3줄 요약해서 대시보드에 표시한다.

### 구현 내용 (커밋: 6a0a854 ~ b578f4b)

- [x] `go/internal/store/db.go` — `news` 테이블 스키마 추가
  ```sql
  CREATE TABLE IF NOT EXISTS news (
      id TEXT PRIMARY KEY, ticker TEXT NOT NULL,
      title TEXT NOT NULL, link TEXT NOT NULL,
      publisher TEXT, published_at INTEGER, fetched_at TEXT NOT NULL
  );
  ```
- [x] `go/internal/store/news.go` — News CRUD
  - `InsertNewsItem` (INSERT OR IGNORE — 중복 무시)
  - `GetNewsForTicker(ticker, limit)` — published_at DESC 정렬
  - `GetLatestNewsFetchedAt(ticker)` — 마지막 수집 시각 조회
- [x] `go/internal/news/crawler.go` — Yahoo Finance 뉴스 수집
  - `Fetch(ticker, baseURL, maxCount)` — `/v1/finance/search` 호출, baseURL 주입으로 테스트 격리
  - `FetchAndStore(ticker, db, baseURL)` — 수집 후 DB 저장
- [x] `go/internal/scheduler/jobs.go` — 백그라운드 스케줄러
  - `New(db, interval, newsBaseURL)` + `Start()` / `Stop()`
  - 서버 시작 시 즉시 1회 수집, 이후 설정 간격(기본 1시간)으로 반복
  - 보유 종목(`status=holding`) 티커 중복 없이 순회
- [x] `go/internal/handler/handler.go` — `POST /brief` 엔드포인트 추가
  - `SetNewsBaseURL(url)` — 테스트용 URL 주입
  - 1시간 이상 된 캐시는 즉시 갱신 후 Claude로 3줄 요약
- [x] `go/internal/handler/templates/brief_result.html` — HTMX 뉴스 브리핑 프래그먼트
- [x] `go/internal/handler/templates/dashboard.html` — "뉴스 브리핑" 섹션 추가 (`hx-post="/brief"`)
- [x] `go/cmd/server/main.go` — 스케줄러 시작/종료 연동

### 실제 완료 기준
- 전체 테스트 PASS (store 9개, news 4개, handler 7개, scheduler 2개)
- 전체 빌드 성공 (`go build ./...`)
- 대시보드 "보유 종목 뉴스 브리핑" 버튼 → HTMX로 Claude 요약 로드

### 미구현 (Phase 4로 이월)
- 주가 자동 수집 스케줄러 (장 마감 후 16:00 KST)
- 패턴 컨텍스트 주입 (단건 분석 시 기존 패턴 요약 전달)
- 주 1회 패턴 리뷰 자동 생성

---

## Phase 4 — 분석 파이프라인 고도화

**목표**: 데이터가 쌓일수록 피드백이 정밀해지는 구조를 완성한다.

### 구현 항목

- [ ] 연속성 있는 패턴 피드백
  - "지난번에도 같은 패턴이었습니다" 형식의 크로스 레퍼런스
- [ ] 습관 변화 트래킹
  - 3개월 전 패턴 vs 최근 패턴 비교
  - "RSI 과매수 매수 비율이 줄었습니다" 등
- [ ] 리포트 히스토리 관리
  - 날짜별 패턴 분석 아카이브
  - 변화 추이 시각화
- [ ] 알림 (선택)
  - 보유 종목 주가 특이 움직임 감지

### 완료 기준
100건의 매매 이력이 쌓였을 때,
"나보다 내 매매 패턴을 더 잘 아는" 수준의 피드백이 나오는가.

---

## 예상 사용자 경험 (Phase 4 완성 시)

```
아침 8:30   대시보드 열기
            → 어제 보유 종목 수익률 현황
            → 오늘 관련 뉴스 3줄 요약
            → 이번 주 특이사항 있으면 알림

매매 후     웹 UI에서 등록
            → 자동으로 지표 수집
            → 단건 분석 리포트 즉시 생성
            → "이번에도 RSI 72에서 매수하셨네요" 크로스 레퍼런스

주 1회      패턴 리뷰 자동 생성
            → 이번 주 매매 습관 요약
            → 이전 주 대비 변화

6개월 후    "당신의 수익률이 높았던 조건"
            "당신이 물린 케이스의 공통점"
            이 문장들이 데이터로 뒷받침된다
```
