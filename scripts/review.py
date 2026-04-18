"""
전체 매매 이력 패턴 분석 모듈.
누적 데이터에서 매수 습관, 익절/손절 패턴을 분석한다.
"""
import os
from datetime import datetime

import anthropic
from dotenv import load_dotenv

from scripts.utils import (
    REPORTS_PATH,
    ensure_cache_dirs,
    get_buy_trades,
    load_profile,
    load_trades,
)

load_dotenv()


def compute_trade_statistics(trades: list[dict]) -> dict:
    """
    매매 이력 집계 통계 계산.

    Returns:
        {
            "total_trades": int,
            "buy_count": int,
            "sell_count": int,
            "holding_count": int,
            "closed_trades": [{"buy_id", "sell_id", "ticker", "pnl_pct", "hold_days"}],
            "markets": {"KRX": int, "NASDAQ": int, ...},
        }
    """
    buy_count = sum(1 for t in trades if t["action"] == "buy")
    sell_count = sum(1 for t in trades if t["action"] == "sell")
    holding_count = sum(1 for t in trades if t.get("status") == "holding")

    buy_map = {t["id"]: t for t in trades if t["action"] == "buy"}
    closed_trades = []
    for t in trades:
        if t["action"] == "sell" and t.get("linked_buy_id"):
            buy_trade = buy_map.get(t["linked_buy_id"])
            if buy_trade:
                pnl_pct = (t["price"] - buy_trade["price"]) / buy_trade["price"] * 100
                buy_date = datetime.strptime(buy_trade["date"], "%Y-%m-%d")
                sell_date = datetime.strptime(t["date"], "%Y-%m-%d")
                hold_days = (sell_date - buy_date).days
                closed_trades.append({
                    "buy_id": buy_trade["id"],
                    "sell_id": t["id"],
                    "ticker": t["ticker"],
                    "pnl_pct": round(pnl_pct, 2),
                    "hold_days": hold_days,
                })

    markets = {}
    for t in trades:
        if t["action"] == "buy":
            m = t.get("market", "unknown")
            markets[m] = markets.get(m, 0) + 1

    return {
        "total_trades": len(trades),
        "buy_count": buy_count,
        "sell_count": sell_count,
        "holding_count": holding_count,
        "closed_trades": closed_trades,
        "markets": markets,
    }


def build_review_prompt(trades: list[dict], stats: dict, profile: dict) -> str:
    """전체 패턴 분석을 위한 Claude 프롬프트 구성"""
    reasons = [t.get("reason", "") for t in trades if t["action"] == "buy"]
    reasons_text = "\n".join(f"- {r}" for r in reasons if r)

    closed = stats["closed_trades"]
    if closed:
        avg_pnl = sum(t["pnl_pct"] for t in closed) / len(closed)
        avg_hold = sum(t["hold_days"] for t in closed) / len(closed)
        pnl_summary = f"평균 수익률: {avg_pnl:.1f}%, 평균 보유 기간: {avg_hold:.0f}일"
    else:
        pnl_summary = "완료된 매매 없음"

    principles = "\n".join(f"- {p}" for p in profile.get("principles", []))

    return f"""당신은 투자자의 매매 패턴을 분석하는 파트너입니다.
개인 투자 성향과 매매 이력을 바탕으로 반복적인 습관 패턴을 찾아줍니다.
매수/매도 추천은 절대 하지 않습니다.

## 전체 매매 통계
- 총 매매: {stats['total_trades']}건 (매수 {stats['buy_count']}건, 매도 {stats['sell_count']}건)
- 현재 보유: {stats['holding_count']}건
- 완료 건 성과: {pnl_summary}
- 시장별 분포: {stats['markets']}

## 매수 이유 전체 목록
{reasons_text}

## 투자자 원칙
{principles if principles else '(원칙 없음)'}

## 패턴 분석 요청
아래 형식으로 3~5개의 패턴 문장을 작성하세요:
"패턴상 당신은 [행동/경향]합니다. [N건 중 N건에서 확인됨]"

분석 항목:
1. 매수 이유의 반복 키워드 (뉴스, 외국인, RSI 등)
2. 매수 타이밍 경향 (급등 추격, 안정 구간 등)
3. 보유 기간 경향
4. 익절/손절 패턴

확정적 예측 표현은 사용하지 마세요. 관찰된 패턴만 기술하세요.
"""


def format_pattern_report(stats: dict, claude_response: str) -> str:
    """패턴 분석 결과를 마크다운 형식으로 포맷팅"""
    now = datetime.now().strftime("%Y-%m-%d %H:%M")
    closed = stats["closed_trades"]

    if closed:
        avg_pnl = sum(t["pnl_pct"] for t in closed) / len(closed)
        avg_hold = sum(t["hold_days"] for t in closed) / len(closed)
        pnl_lines = "\n".join(
            f"| {t['ticker']} | {t['pnl_pct']:+.1f}% | {t['hold_days']}일 |"
            for t in closed
        )
        closed_section = f"""
| 종목 | 수익률 | 보유기간 |
|------|--------|---------|
{pnl_lines}

**평균:** 수익률 {avg_pnl:+.1f}%, 보유기간 {avg_hold:.0f}일
"""
    else:
        closed_section = "완료된 매매 없음"

    return f"""# 매매 패턴 분석 리포트

생성일시: {now}

---

## 전체 통계

| 항목 | 값 |
|------|-----|
| 총 매매 | {stats['total_trades']}건 |
| 매수 | {stats['buy_count']}건 |
| 매도 | {stats['sell_count']}건 |
| 현재 보유 | {stats['holding_count']}건 |

## 완료된 매매 성과
{closed_section}

---

## 패턴 분석

{claude_response}

---

*이 리포트는 과거 데이터 기반 참고 자료입니다. 투자 자문이 아닙니다.*
"""


def run_review() -> str:
    """전체 패턴 분석 실행. 리포트 파일 경로 반환."""
    ensure_cache_dirs()

    trades = load_trades()
    profile = load_profile()

    if not trades:
        raise ValueError("매매 이력이 없습니다.")

    stats = compute_trade_statistics(trades)

    print("[review] Claude API로 패턴 분석 중...")
    client = anthropic.Anthropic(api_key=os.environ["ANTHROPIC_API_KEY"])
    prompt = build_review_prompt(trades, stats, profile)

    message = client.messages.create(
        model="claude-sonnet-4-6",
        max_tokens=1024,
        messages=[{"role": "user", "content": prompt}],
    )
    pattern_text = message.content[0].text

    report = format_pattern_report(stats, pattern_text)
    date_str = datetime.now().strftime("%Y-%m-%d")
    report_path = REPORTS_PATH / f"pattern-{date_str}.md"
    report_path.write_text(report, encoding="utf-8")

    print(f"[done] 리포트 저장: {report_path}")
    return str(report_path)


if __name__ == "__main__":
    result = run_review()
    print(f"\n리포트: {result}")
