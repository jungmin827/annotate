import pytest
from pathlib import Path
from scripts.analyze import (
    build_analysis_prompt,
    format_report,
    get_report_filename,
)

FIXTURES_DIR = Path(__file__).parent / "fixtures"


SAMPLE_TRADE = {
    "id": "test_001",
    "ticker": "005930",
    "name": "삼성전자",
    "market": "KRX",
    "action": "buy",
    "price": 70000,
    "quantity": 10,
    "date": "2024-01-15",
    "reason": "반도체 사이클 회복 기대. 외국인 순매수.",
    "status": "holding",
}

SAMPLE_SUMMARY = {
    "date": "2024-01-15",
    "close": 70500,
    "rsi": 62.3,
    "volume_ratio": 1.2,
    "pct_change": 0.8,
    "cumulative_5d": 3.2,
    "signal": "특이사항없음",
}


def test_build_analysis_prompt_contains_trade_info():
    prompt = build_analysis_prompt(SAMPLE_TRADE, SAMPLE_SUMMARY, profile={})
    assert "005930" in prompt or "삼성전자" in prompt
    assert "반도체 사이클 회복" in prompt
    assert "62.3" in prompt or "RSI" in prompt


def test_build_analysis_prompt_no_buy_sell_advice():
    prompt = build_analysis_prompt(SAMPLE_TRADE, SAMPLE_SUMMARY, profile={})
    assert "사세요" not in prompt
    assert "파세요" not in prompt


def test_get_report_filename():
    filename = get_report_filename("test_001", "005930", "2024-01-15")
    assert filename == "test_001-005930-2024-01-15.md"


def test_format_report_contains_sections():
    mock_claude_response = "이 매매는 RSI 62.3으로 정상 구간에서 진입하였습니다."
    report = format_report(SAMPLE_TRADE, SAMPLE_SUMMARY, mock_claude_response)
    assert "## 매매 정보" in report
    assert "## 기술적 상태" in report
    assert "## 분석" in report
    assert "005930" in report or "삼성전자" in report


def test_format_report_no_prediction_language():
    mock_response = "반드시 상승합니다. 사세요."
    report = format_report(SAMPLE_TRADE, SAMPLE_SUMMARY, mock_response)
    header_section = report.split("## 분석")[0]
    assert "반드시" not in header_section
    assert "사세요" not in header_section
