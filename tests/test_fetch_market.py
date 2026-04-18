import pytest
import pandas as pd
from pathlib import Path
from unittest.mock import patch, MagicMock
from scripts.fetch_market import (
    normalize_ticker,
    load_ohlcv_from_cache,
    save_ohlcv_to_cache,
    fetch_ohlcv,
    calculate_indicators,
)

TMP_CACHE = Path("/tmp/test_market_cache")


def make_sample_ohlcv() -> pd.DataFrame:
    """테스트용 OHLCV DataFrame 생성"""
    dates = pd.date_range("2024-01-10", periods=10, freq="B")
    return pd.DataFrame({
        "Open": [70000, 71000, 72000, 71500, 73000, 74000, 73500, 75000, 74500, 76000],
        "High": [71000, 72000, 73000, 72500, 74000, 75000, 74500, 76000, 75500, 77000],
        "Low":  [69000, 70000, 71000, 70500, 72000, 73000, 72500, 74000, 73500, 75000],
        "Close":[70500, 71500, 72500, 71000, 73500, 74500, 74000, 75500, 75000, 76500],
        "Volume":[1000000]*10,
    }, index=dates)


def test_normalize_ticker_krx():
    assert normalize_ticker("005930", "KRX") == "005930.KS"


def test_normalize_ticker_nasdaq():
    assert normalize_ticker("NVDA", "NASDAQ") == "NVDA"


def test_normalize_ticker_nyse():
    assert normalize_ticker("AAPL", "NYSE") == "AAPL"


def test_save_and_load_ohlcv_cache():
    TMP_CACHE.mkdir(parents=True, exist_ok=True)
    df = make_sample_ohlcv()
    save_ohlcv_to_cache(df, "005930", TMP_CACHE)
    loaded = load_ohlcv_from_cache("005930", TMP_CACHE)
    assert loaded is not None
    assert len(loaded) == len(df)
    assert list(loaded.columns) == list(df.columns)


def test_load_ohlcv_from_cache_returns_none_if_missing():
    result = load_ohlcv_from_cache("NONEXISTENT_TICKER_XYZ", TMP_CACHE)
    assert result is None


def test_calculate_indicators_has_required_columns():
    df = make_sample_ohlcv()
    result = calculate_indicators(df)
    assert "RSI_14" in result.columns
    assert "volume_ratio" in result.columns
    assert "pct_change" in result.columns


def test_calculate_indicators_rsi_range():
    df = make_sample_ohlcv()
    result = calculate_indicators(df)
    rsi_values = result["RSI_14"].dropna()
    assert all(0 <= v <= 100 for v in rsi_values)


@patch("scripts.fetch_market.yf.Ticker")
def test_fetch_ohlcv_calls_yfinance(mock_ticker_cls):
    mock_ticker = MagicMock()
    mock_ticker.history.return_value = make_sample_ohlcv()
    mock_ticker_cls.return_value = mock_ticker

    result = fetch_ohlcv("NVDA", "NASDAQ", start="2024-01-10", end="2024-01-20", use_cache=False)
    assert result is not None
    assert len(result) > 0
    mock_ticker.history.assert_called_once()
