import pytest
from pathlib import Path
from scripts.review import (
    compute_trade_statistics,
    build_review_prompt,
    format_pattern_report,
)

FIXTURES_DIR = Path(__file__).parent / "fixtures"


SAMPLE_TRADES = [
    {
        "id": "t1", "ticker": "005930", "name": "삼성전자", "market": "KRX",
        "action": "buy", "price": 70000, "quantity": 10,
        "date": "2024-01-15", "reason": "외국인 순매수", "status": "closed",
    },
    {
        "id": "t2", "ticker": "005930", "name": "삼성전자", "market": "KRX",
        "action": "sell", "price": 77000, "quantity": 10,
        "date": "2024-02-20", "reason": "목표가 달성", "status": "closed",
        "linked_buy_id": "t1",
    },
    {
        "id": "t3", "ticker": "NVDA", "name": "NVIDIA", "market": "NASDAQ",
        "action": "buy", "price": 500.0, "quantity": 5,
        "date": "2024-03-01", "reason": "AI 수혜주", "status": "holding",
    },
]


def test_compute_trade_statistics_basic():
    stats = compute_trade_statistics(SAMPLE_TRADES)
    assert stats["total_trades"] == 3
    assert stats["buy_count"] == 2
    assert stats["sell_count"] == 1
    assert stats["holding_count"] == 1


def test_compute_trade_statistics_pnl():
    stats = compute_trade_statistics(SAMPLE_TRADES)
    assert "closed_trades" in stats
    assert len(stats["closed_trades"]) == 1
    pnl = stats["closed_trades"][0]["pnl_pct"]
    assert abs(pnl - 10.0) < 0.1  # 10% 수익


def test_build_review_prompt_contains_stats():
    stats = compute_trade_statistics(SAMPLE_TRADES)
    prompt = build_review_prompt(SAMPLE_TRADES, stats, profile={})
    assert "3" in prompt  # 전체 매매 수
    assert "매수" in prompt
    assert "패턴" in prompt


def test_format_pattern_report_structure():
    stats = compute_trade_statistics(SAMPLE_TRADES)
    mock_response = "패턴상 당신은 외국인 순매수 이후 매수하는 경향이 있습니다."
    report = format_pattern_report(stats, mock_response)
    assert "## 전체 통계" in report
    assert "## 패턴 분석" in report
    assert mock_response in report
