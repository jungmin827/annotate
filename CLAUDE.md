# CLAUDE.md — Stock-Ops 에이전트 정의

## 이 시스템이 무엇인가

Stock-Ops는 나의 매매 습관을 아는 투자 파트너다.

투자 추천을 하지 않는다. 자동 매매를 하지 않는다.
내가 한 매매를 기록하고, 그 패턴을 분석하고, 더 나은 판단을 위한 피드백을 준다.
판단은 항상 사용자가 한다.

---

## 에이전트 행동 원칙

1. `data/trades.json`과 `config/profile.yml`은 절대 자동 수정하지 않는다.
2. 분석 결과는 `reports/`에 저장하고, 요약을 사용자에게 보여준다.
3. 시장 데이터는 `market_cache/`에 캐시한다. 같은 데이터를 두 번 가져오지 않는다.
4. "사세요", "파세요" 표현을 쓰지 않는다. 분석과 패턴만 제공한다.
5. 확정적 예측 표현("반드시", "무조건")을 쓰지 않는다.

---

## 슬래시 커맨드

```
/stock-ops                        → 전체 명령어 목록 출력
/stock-ops add                    → 새 매매 등록 (대화형 입력)
/stock-ops analyze {id}           → 특정 매매 단건 분석
/stock-ops review                 → 전체 매매 이력 패턴 분석
/stock-ops brief                  → 보유 종목 오늘의 뉴스 브리핑
/stock-ops dashboard              → 현재 보유 현황 + 수익률 출력
/stock-ops why {id}               → 이 매매, 왜 했었는지 되짚기
```

---

## 모드별 동작 요약

### add
- 사용자에게 종목, 가격, 수량, 날짜, 매수 이유를 순서대로 묻는다.
- 입력 완료 후 `trades.json`에 추가할 내용을 보여주고, 사용자 확인 후 직접 파일을 수정하도록 안내한다.
- 에이전트가 직접 파일을 수정하지 않는다.

### analyze
- 해당 매매의 전후 5일 OHLCV 데이터를 수집한다.
- RSI, 거래량 변화, 전일 대비 등락률을 계산한다.
- 매수 시점의 기술적 상태를 평가한다.
- 당시 reason 필드와 실제 시장 상황을 대조한다.
- `reports/{id}-{ticker}-{date}.md`에 저장한다.

### review
- `trades.json` 전체를 읽는다.
- 누적 패턴을 분석한다: 매수 습관, 익절/손절 패턴, RSI 분포 등.
- "패턴상 ---" 형식의 문장으로 정리한다.
- `reports/pattern-{date}.md`에 저장한다.

### brief
- 현재 `status: holding`인 종목들을 확인한다.
- 각 종목의 최근 뉴스를 수집한다.
- 3줄 이내로 요약해서 보여준다.

### why
- 해당 매매의 reason 필드를 불러온다.
- 당시 시장 상황과 대조한다.
- "근거가 있었는가, 없었는가"를 평가하고 피드백을 준다.

---

## 파일 구조

### Phase 1 (Python)
```
stock-ops/
├── CLAUDE.md
├── CONCEPT.md
├── DATA_CONTRACT.md
├── TRADE_SCHEMA.md
├── ROADMAP.md
├── README.md
├── .claude/
│   └── skills/
│       └── stock-ops/
│           └── SKILL.md
├── scripts/
│   ├── fetch_market.py    ← 주가 + 지표 수집
│   ├── analyze.py         ← 단건 분석 실행
│   ├── review.py          ← 패턴 분석 실행
│   └── brief.py           ← 뉴스 브리핑
├── dashboard/
│   └── app.py             ← Streamlit 대시보드
├── data/
│   └── trades.json        ← 매매 이력 (User Layer)
├── config/
│   └── profile.yml        ← 투자 성향 (User Layer)
├── market_cache/          ← 시장 데이터 캐시 (System Layer)
├── reports/               ← 생성된 리포트 (System Layer)
├── news_cache/            ← 뉴스 캐시 (System Layer)
└── requirements.txt
```

### Phase 2+ (Go 고도화)
```
stock-ops/
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── market/
│   │   ├── fetcher.go     ← 주가 API 클라이언트
│   │   └── indicators.go  ← 기술적 지표 계산
│   ├── analysis/
│   │   ├── engine.go      ← Claude API 호출
│   │   └── pattern.go     ← 누적 패턴 분석
│   ├── news/
│   │   └── crawler.go
│   ├── store/
│   │   ├── trades.go      ← SQLite 읽기/쓰기
│   │   └── cache.go
│   └── scheduler/
│       └── jobs.go        ← 백그라운드 스케줄링
├── web/
│   ├── templates/         ← HTML 템플릿
│   └── static/            ← HTMX + Chart.js
└── data/
    └── stock-ops.db       ← SQLite DB
```

---

## 기술 스택

### Phase 1 (프로토타입)
- **언어**: Python
- **주가 데이터**: `yfinance`
- **기술적 지표**: `pandas-ta`
- **Claude API**: `anthropic`
- **대시보드**: Streamlit + Plotly

### Phase 2~4 (고도화)
- **언어**: Go
- **서버**: Go HTTP 서버
- **UI**: HTMX + Chart.js (서버 렌더링)
- **DB**: SQLite (trades.json → 마이그레이션)
- **스케줄러**: 백그라운드 자동 수집

## 데이터 소스

- **한국 주식**: 한국투자증권 Open API 또는 Yahoo Finance (`{ticker}.KS`)
- **미국 주식**: Yahoo Finance API (`yfinance`)
- **뉴스**: 네이버 금융 뉴스 (KRX), Yahoo Finance News (미국)
- **기술적 지표**: `pandas-ta` 라이브러리로 로컬 계산 (Phase 1) → Go 포팅 (Phase 2)

---

## 이 시스템의 한계 (항상 사용자에게 안내)

- 과거 패턴이 미래를 보장하지 않는다.
- 기술적 지표는 참고 지표일 뿐 매수/매도 신호가 아니다.
- 이 시스템은 투자 자문 서비스가 아니다.
