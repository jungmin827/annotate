# DATA_CONTRACT.md — 데이터 계약

## 원칙

두 가지 레이어가 존재한다.

**User Layer** — 절대 자동 수정되지 않는다. 오직 사용자가 직접 편집한다.
**System Layer** — 에이전트가 자동으로 생성·갱신한다.

---

## User Layer (절대 자동 수정 금지)

| 파일 | 설명 |
|---|---|
| `data/trades.json` | 나의 매매 이력. 이 파일이 모든 분석의 원천이다. |
| `config/profile.yml` | 나의 투자 성향, 목표 수익률, 투자 원칙 |

이 파일들은 에이전트가 읽기만 한다. **절대 덮어쓰지 않는다.**
업데이트가 필요한 경우 에이전트는 사용자에게 내용을 제안하고, 직접 편집하도록 안내한다.

---

## System Layer (자동 생성·갱신)

| 경로 | 설명 |
|---|---|
| `market_cache/{ticker}/` | API로 가져온 주가 OHLCV 데이터 캐시 |
| `market_cache/{ticker}/indicators.json` | 계산된 기술적 지표 (RSI, MACD 등) |
| `reports/{id}-{ticker}-{date}.md` | 생성된 매매 분석 리포트 |
| `reports/pattern-{date}.md` | 누적 패턴 분석 리포트 |
| `news_cache/{ticker}/` | 크롤링된 관련 뉴스 |

---

## 데이터 흐름

```
사용자가 trades.json에 매매 등록
    ↓
에이전트가 해당 종목·날짜의 시장 데이터 수집 (market_cache/)
    ↓
기술적 지표 계산 및 저장
    ↓
관련 뉴스 수집 (news_cache/)
    ↓
단건 분석 리포트 생성 (reports/)
    ↓
누적 이력 기반 패턴 분석 업데이트
```

---

## 프라이버시

- 모든 데이터는 로컬에 저장된다.
- `trades.json`과 `profile.yml`의 내용은 분석 시 Claude API로 전송된다.
- 외부 서버나 별도 데이터베이스는 없다.
- 분석에 사용된 데이터는 사용자의 AI 제공자(Anthropic) 정책을 따른다.
