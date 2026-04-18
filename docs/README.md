# Stock-Ops

> 나의 매매 습관을 아는 투자 파트너.

판단은 내가 한다. Stock-Ops는 내가 더 나은 판단을 하도록 돕는 거울이다.

---

## 왜 만들었나

근거 없는 매매 → 물림 → 원인을 모름 → 반복.

이 루프를 끊기 위해 만들었다.
뉴스도 안 보고, 차트도 잘 모르고, 그냥 유망해 보여서 샀다가 물리는 패턴.
그 패턴을 데이터로 보여주고, 피드백을 주는 시스템이다.

---

## 무엇을 하는가

```
/stock-ops add          → 매매 등록
/stock-ops analyze {id} → 특정 매매 기술적 분석
/stock-ops review       → 전체 매매 패턴 분석
/stock-ops brief        → 보유 종목 뉴스 브리핑
/stock-ops why {id}     → 이 매매 왜 했었는지 되짚기
/stock-ops dashboard    → 보유 현황 + 수익률
```

---

## 무엇을 하지 않는가

- 투자 추천 없음
- 자동 매매 없음
- "사세요 / 파세요" 없음

---

## 세계관

처음엔 단건 분석만 가능하다.
30건이 쌓이면 습관을 말해준다.
100건이 쌓이면 나보다 내 매매 패턴을 더 잘 안다.

**이건 앱이 아니다. 파트너를 키우는 것이다.**

---

## 시작하기

**1. 설정**
```bash
cp config/profile.example.yml config/profile.yml
# profile.yml을 편집하여 투자 성향 입력
```

**2. 첫 매매 등록**
```
/stock-ops add
```

**3. 분석 시작**
```
/stock-ops analyze trade_001
```

---

## 기술 스택

### Phase 1 — Python 프로토타입
```
yfinance      → 주가 OHLCV 데이터
pandas-ta     → RSI, MACD, 볼린저밴드
anthropic     → Claude API
streamlit     → 대시보드
plotly        → 차트
```

### Phase 2+ — Go 고도화
```
Go HTTP 서버  → 백엔드
HTMX          → 서버 렌더링 UI
Chart.js      → 차트
SQLite        → 로컬 DB
Go Scheduler  → 자동 데이터 수집
```

---

## 문서

| 문서 | 설명 |
|---|---|
| `CONCEPT.md` | 세계관과 포지셔닝 |
| `DATA_CONTRACT.md` | 데이터 계약 (무엇이 자동 수정되고 안 되는가) |
| `TRADE_SCHEMA.md` | 매매 데이터 스키마 |
| `CLAUDE.md` | 에이전트 행동 정의 + 기술 스택 |
| `ROADMAP.md` | Phase별 개발 로드맵 |
| `.claude/skills/stock-ops/SKILL.md` | 모드별 상세 동작 |

---

## 면책

Stock-Ops는 투자 자문 서비스가 아닙니다.
모든 분석은 참고용이며, 투자 결정의 책임은 사용자에게 있습니다.
