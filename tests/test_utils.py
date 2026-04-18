import pytest
from pathlib import Path
from scripts.utils import load_trades, load_profile, get_trade_by_id, get_holding_trades

FIXTURES_DIR = Path(__file__).parent / "fixtures"


def test_load_trades_returns_list():
    trades = load_trades(FIXTURES_DIR / "trades_sample.json")
    assert isinstance(trades, list)
    assert len(trades) == 2


def test_load_trades_has_required_fields():
    trades = load_trades(FIXTURES_DIR / "trades_sample.json")
    required = {"id", "ticker", "name", "market", "action", "price", "quantity", "date", "status", "reason"}
    for trade in trades:
        assert required.issubset(trade.keys()), f"Missing fields in: {trade}"


def test_get_trade_by_id_found():
    trades = load_trades(FIXTURES_DIR / "trades_sample.json")
    trade = get_trade_by_id(trades, "test_001")
    assert trade["ticker"] == "005930"
    assert trade["price"] == 70000


def test_get_trade_by_id_not_found():
    trades = load_trades(FIXTURES_DIR / "trades_sample.json")
    trade = get_trade_by_id(trades, "nonexistent")
    assert trade is None


def test_get_holding_trades():
    trades = load_trades(FIXTURES_DIR / "trades_sample.json")
    holding = get_holding_trades(trades)
    # fixtures에는 holding이 없음 (모두 closed)
    assert isinstance(holding, list)
    assert all(t["status"] == "holding" for t in holding)


def test_load_profile(tmp_path):
    profile_file = tmp_path / "profile.yml"
    profile_file.write_text("name: '테스터'\ninvestment:\n  style: '단기'\n")
    profile = load_profile(profile_file)
    assert profile["name"] == "테스터"
    assert profile["investment"]["style"] == "단기"
