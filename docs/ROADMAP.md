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

## Phase 2 — Go 서버 전환

**목표**: Python 스크립트를 Go HTTP 서버로 전환한다. 항상 켜져 있는 대시보드.

### 스택
```
Go + HTMX + Chart.js
```

### 구현 항목

- [ ] `cmd/server/main.go` — HTTP 서버 진입점
- [ ] `internal/market/fetcher.go` — 주가 API 클라이언트
- [ ] `internal/market/indicators.go` — RSI, MACD 등 Go 구현
- [ ] `internal/analysis/engine.go` — Claude API 호출 레이어
- [ ] `web/templates/` — 대시보드 HTML 템플릿
- [ ] `web/static/` — HTMX + Chart.js

### 완료 기준
브라우저에서 대시보드 접근 가능.
매매 등록 → 분석 리포트 생성까지 웹 UI에서 완결.

---

## Phase 3 — 자동화 + DB 마이그레이션

**목표**: 수동으로 명령어를 치지 않아도 데이터가 쌓인다.

### 구현 항목

- [ ] `internal/scheduler/jobs.go` — 백그라운드 스케줄러
  - 장 마감 후 보유 종목 주가 자동 수집 (16:00 KST)
  - 매일 오전 뉴스 브리핑 자동 생성 (08:30 KST)
  - 주 1회 패턴 리뷰 자동 생성 (일요일 저녁)
- [ ] `internal/store/trades.go` — JSON → SQLite 마이그레이션
- [ ] 분석 파이프라인에 패턴 컨텍스트 주입
  - 단건 분석 시 "이 사람의 기존 패턴" 요약을 Claude에게 함께 전달

### 완료 기준
아침에 대시보드를 열면 어제 데이터와 오늘 뉴스가 자동으로 준비되어 있다.

### SQLite 스키마 (예상)
```sql
CREATE TABLE trades (
    id TEXT PRIMARY KEY,
    ticker TEXT,
    name TEXT,
    market TEXT,
    action TEXT,
    price REAL,
    quantity INTEGER,
    date TEXT,
    reason TEXT,
    status TEXT,
    rsi_at_trade REAL,
    pct_change_at_trade REAL,
    volume_ratio_at_trade REAL,
    linked_id TEXT
);

CREATE TABLE market_cache (
    ticker TEXT,
    date TEXT,
    open REAL, high REAL, low REAL, close REAL, volume INTEGER,
    rsi REAL, macd REAL,
    PRIMARY KEY (ticker, date)
);
```

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
