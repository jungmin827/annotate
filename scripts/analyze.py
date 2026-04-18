"""
단건 매매 분석 모듈.
특정 trade_id의 매매를 기술적 지표와 Claude API를 통해 분석한다.
"""
import os
from datetime import datetime
from pathlib import Path

import anthropic
from dotenv import load_dotenv

from scripts.fetch_market import get_market_data_around_date, summarize_on_date
from scripts.utils import (
    REPORTS_PATH,
    ensure_cache_dirs,
    get_trade_by_id,
    load_profile,
    load_trades,
)

load_dotenv()


def build_analysis_prompt(trade: dict, summary: dict, profile: dict) -> str:
    """Claude API에 전달할 분석 프롬프트 구성"""
    principles = "\n".join(f"- {p}" for p in profile.get("principles", []))

    return f"""당신은 투자자의 매매 기록을 분석하는 파트너입니다.
매수/매도 추천은 절대 하지 않습니다. 과거 데이터를 바탕으로 기술적 상태와 매수 근거의 타당성만 평가합니다.

## 매매 정보
- 종목: {trade['name']} ({trade['ticker']}, {trade['market']})
- 행동: {trade['action']}
- 가격: {trade['price']:,}
- 수량: {trade['quantity']}
- 날짜: {trade['date']}
- 매수 이유: {trade['reason']}

## 매수 시점 기술적 상태
- 종가: {summary['close']:,}
- RSI(14): {summary['rsi']}
- 전일 대비 등락률: {summary['pct_change']}%
- 거래량/5일평균: {summary['volume_ratio']}배
- 5일 누적 상승률: {summary['cumulative_5d']}%
- 판단: {summary['signal']}

## 투자자 원칙
{principles if principles else '(원칙 없음)'}

## 분석 요청
1. 매수 시점의 기술적 상태를 평가하세요 (과매수/정상/과매도 구간 여부).
2. 매수 이유(reason)가 실제 시장 상황과 일치하는지 평가하세요.
3. 근거 타당성을 "근거 충분", "근거 약함", "근거 부족" 중 하나로 판단하세요.
4. 이 매매에서 개선할 수 있는 점을 1~2문장으로 제시하세요.

확정적 예측("반드시", "무조건")이나 매수/매도 추천 표현은 사용하지 마세요.
패턴 관찰과 사실 기반으로만 작성하세요.
"""


def format_report(trade: dict, summary: dict, claude_response: str) -> str:
    """분석 결과를 마크다운 리포트 형식으로 포맷팅"""
    now = datetime.now().strftime("%Y-%m-%d %H:%M")
    return f"""# 매매 분석 리포트 — {trade['name']} ({trade['id']})

생성일시: {now}

---

## 매매 정보

| 항목 | 내용 |
|------|------|
| 종목 | {trade['name']} ({trade['ticker']}) |
| 시장 | {trade['market']} |
| 행동 | {trade['action']} |
| 가격 | {trade['price']:,} |
| 수량 | {trade['quantity']} |
| 날짜 | {trade['date']} |
| 상태 | {trade['status']} |

**매수 이유:** {trade['reason']}

---

## 기술적 상태 (매수 시점)

| 지표 | 값 |
|------|----|
| 종가 | {summary['close']:,} |
| RSI(14) | {summary['rsi']} |
| 전일 대비 | {summary['pct_change']}% |
| 거래량 비율 | {summary['volume_ratio']}배 |
| 5일 누적 | {summary['cumulative_5d']}% |
| 신호 | {summary['signal']} |

---

## 분석

{claude_response}

---

*이 리포트는 과거 데이터 기반 참고 자료입니다. 투자 자문이 아닙니다.*
"""


def get_report_filename(trade_id: str, ticker: str, date: str) -> str:
    return f"{trade_id}-{ticker}-{date}.md"


def analyze_trade(trade_id: str) -> str:
    """
    단건 매매 분석 실행.

    Returns:
        저장된 리포트 파일 경로
    """
    ensure_cache_dirs()

    trades = load_trades()
    trade = get_trade_by_id(trades, trade_id)
    if trade is None:
        raise ValueError(f"매매 ID '{trade_id}'를 찾을 수 없습니다.")

    profile = load_profile()

    print(f"[fetch] {trade['name']} ({trade['ticker']}) 시장 데이터 수집 중...")
    df = get_market_data_around_date(trade["ticker"], trade["market"], trade["date"])
    if df is None:
        raise RuntimeError(f"시장 데이터를 가져올 수 없습니다: {trade['ticker']}")

    summary = summarize_on_date(df, trade["date"])

    print("[analyze] Claude API로 분석 중...")
    client = anthropic.Anthropic(api_key=os.environ["ANTHROPIC_API_KEY"])
    prompt = build_analysis_prompt(trade, summary, profile)

    message = client.messages.create(
        model="claude-sonnet-4-6",
        max_tokens=1024,
        messages=[{"role": "user", "content": prompt}],
    )
    analysis_text = message.content[0].text

    report = format_report(trade, summary, analysis_text)
    filename = get_report_filename(trade_id, trade["ticker"], trade["date"])
    report_path = REPORTS_PATH / filename
    report_path.write_text(report, encoding="utf-8")

    print(f"[done] 리포트 저장: {report_path}")
    return str(report_path)


if __name__ == "__main__":
    import sys
    if len(sys.argv) < 2:
        print("사용법: python -m scripts.analyze <trade_id>")
        sys.exit(1)
    result = analyze_trade(sys.argv[1])
    print(f"\n리포트: {result}")
