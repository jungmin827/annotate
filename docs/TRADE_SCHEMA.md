# TRADE_SCHEMA.md — 매매 데이터 스키마

## trades.json 구조

```json
[
  {
    "id": "trade_001",
    "ticker": "005930",
    "name": "삼성전자",
    "market": "KRX",
    "action": "buy",
    "price": 72000,
    "quantity": 10,
    "date": "2025-04-10",
    "time": "10:23",
    "reason": "반도체 사이클 회복 기대, HBM 수혜 기대. 뉴스는 따로 안 봤고 그냥 요즘 AI 반도체 얘기가 많아서.",
    "status": "holding",
    "linked_sell_id": null
  },
  {
    "id": "trade_002",
    "ticker": "005930",
    "name": "삼성전자",
    "market": "KRX",
    "action": "sell",
    "price": 75000,
    "quantity": 10,
    "date": "2025-04-18",
    "time": "14:10",
    "reason": "목표 수익률 도달. 더 오를 것 같기도 했지만 익절.",
    "status": "closed",
    "linked_buy_id": "trade_001"
  }
]
```

---

## 필드 정의

| 필드 | 타입 | 필수 | 설명 |
|---|---|---|---|
| `id` | string | ✅ | 고유 식별자. `trade_001` 형식 |
| `ticker` | string | ✅ | 종목 코드. KRX는 6자리 숫자, 미국은 심볼 (AAPL 등) |
| `name` | string | ✅ | 종목명 |
| `market` | string | ✅ | `KRX` 또는 `NASDAQ` 또는 `NYSE` |
| `action` | string | ✅ | `buy` 또는 `sell` |
| `price` | number | ✅ | 매수/매도 단가 |
| `quantity` | number | ✅ | 수량 |
| `date` | string | ✅ | `YYYY-MM-DD` 형식 |
| `time` | string | ❌ | `HH:MM` 형식. 없으면 생략 가능 |
| `reason` | string | ✅ | **이 필드가 핵심.** 왜 샀는지/팔았는지를 자유롭게 적는다. 짧아도 된다. |
| `status` | string | ✅ | `holding` (보유중) / `closed` (매도 완료) / `loss` (손절) |
| `linked_sell_id` | string | ❌ | 매수 기록에서 연결된 매도 id |
| `linked_buy_id` | string | ❌ | 매도 기록에서 연결된 매수 id |

---

## reason 필드 작성 가이드

이 필드가 나중에 회고 피드백의 핵심 재료가 된다.
잘 쓸수록 피드백의 질이 올라간다. 하지만 처음엔 짧아도 괜찮다.

**나쁜 예:**
```
"그냥 좋아 보여서"
```

**좋은 예:**
```
"AI 반도체 수요 증가 뉴스를 봤고, 요즘 HBM 얘기가 많아서 수혜주라고 생각했다.
전날 3% 올랐고, 오늘도 올라가길래 탔다. 딱히 차트는 안 봤음."
```

처음엔 짧게 써도 된다. 데이터가 쌓이면서 자연스럽게 더 구체적으로 쓰게 된다.

---

## profile.yml 구조

```yaml
name: "내 이름 또는 닉네임"

investment:
  style: "단기 스윙 위주"          # 자유 텍스트
  goal_return_pct: 15              # 목표 수익률 (%)
  loss_cut_pct: -8                 # 손절 기준 (%)
  preferred_markets:
    - KRX
    - NASDAQ

principles:
  - "뉴스 없이는 매수하지 않는다"
  - "RSI 70 이상에서는 신규 매수 자제"
  - "익절은 목표가 도달 시 분할 매도"

notes: |
  투자 경험 1년차. 반도체, 2차전지, AI 관련주 관심.
  아직 기술적 분석은 잘 모름.
```
