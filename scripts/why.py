"""
매매 회고 모드.
특정 매매의 reason을 당시 시장 상황과 대조하여 근거를 평가한다.
"""
import os

import anthropic
from dotenv import load_dotenv

from scripts.fetch_market import get_market_data_around_date, summarize_on_date
from scripts.utils import ensure_cache_dirs, get_trade_by_id, load_profile, load_trades

load_dotenv()


def build_why_prompt(trade: dict, summary: dict, profile: dict) -> str:
    principles = "\n".join(f"- {p}" for p in profile.get("principles", []))
    return f"""당신은 투자자의 매매 회고를 돕는 파트너입니다.
매수/매도 추천은 하지 않습니다. 당시의 판단 근거를 사실 기반으로 평가합니다.

## 당시 매매
- 종목: {trade['name']} ({trade['ticker']})
- 날짜: {trade['date']}
- 행동: {trade['action']}
- 가격: {trade['price']:,}
- 매수 이유: {trade['reason']}
- 현재 상태: {trade['status']}

## 당시 시장 상황
- RSI(14): {summary['rsi']}
- 전일 대비: {summary['pct_change']}%
- 거래량 비율: {summary['volume_ratio']}배
- 신호: {summary['signal']}

## 투자자 원칙
{principles if principles else '(원칙 없음)'}

## 평가 요청
1. 매수 이유에 실질적 근거가 있었는가를 "근거 충분", "근거 약함", "근거 부족" 중 하나로 평가하세요.
2. 투자자 원칙과 일치하는 매매였는가를 평가하세요.
3. 이 매매에서 기억할 점을 1문장으로 정리하세요.

확정적 예측 표현이나 추천 표현은 사용하지 마세요.
"""


def run_why(trade_id: str) -> str:
    """매매 회고 실행"""
    ensure_cache_dirs()

    trades = load_trades()
    trade = get_trade_by_id(trades, trade_id)
    if trade is None:
        return f"매매 ID '{trade_id}'를 찾을 수 없습니다."

    profile = load_profile()

    df = get_market_data_around_date(trade["ticker"], trade["market"], trade["date"])
    if df is None:
        return "시장 데이터를 가져올 수 없습니다."

    summary = summarize_on_date(df, trade["date"])

    client = anthropic.Anthropic(api_key=os.environ["ANTHROPIC_API_KEY"])
    prompt = build_why_prompt(trade, summary, profile)

    message = client.messages.create(
        model="claude-sonnet-4-6",
        max_tokens=512,
        messages=[{"role": "user", "content": prompt}],
    )

    return f"""## {trade['name']} ({trade['id']}) — 매매 회고

**매수 이유:** {trade['reason']}
**당시 RSI:** {summary['rsi']} | **등락률:** {summary['pct_change']}%

---

{message.content[0].text}
"""


if __name__ == "__main__":
    import sys
    if len(sys.argv) < 2:
        print("사용법: python -m scripts.why <trade_id>")
        sys.exit(1)
    result = run_why(sys.argv[1])
    print(result)
