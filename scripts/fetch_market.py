"""
시장 데이터 수집 모듈.
Yahoo Finance에서 OHLCV 데이터를 가져오고 market_cache에 저장한다.
같은 데이터를 두 번 가져오지 않는다.
"""
from datetime import datetime, timedelta
from pathlib import Path
from typing import Optional

import pandas as pd
import ta.momentum
import ta.volume
import yfinance as yf

from scripts.utils import MARKET_CACHE_PATH, ensure_cache_dirs


def normalize_ticker(ticker: str, market: str) -> str:
    """KRX 종목은 Yahoo Finance .KS 형식으로 변환"""
    if market == "KRX":
        return f"{ticker}.KS"
    return ticker


def load_ohlcv_from_cache(ticker: str, cache_dir: Path = MARKET_CACHE_PATH) -> Optional[pd.DataFrame]:
    """캐시에서 OHLCV 데이터 로드. 없으면 None 반환."""
    cache_file = cache_dir / f"{ticker}_ohlcv.csv"
    if not cache_file.exists():
        return None
    df = pd.read_csv(cache_file, index_col=0, parse_dates=True)
    return df


def save_ohlcv_to_cache(df: pd.DataFrame, ticker: str, cache_dir: Path = MARKET_CACHE_PATH):
    """OHLCV 데이터를 캐시에 저장"""
    cache_dir.mkdir(parents=True, exist_ok=True)
    cache_file = cache_dir / f"{ticker}_ohlcv.csv"
    df.to_csv(cache_file)


def fetch_ohlcv(
    ticker: str,
    market: str,
    start: str,
    end: str,
    use_cache: bool = True,
    cache_dir: Path = MARKET_CACHE_PATH,
) -> Optional[pd.DataFrame]:
    """
    OHLCV 데이터 조회. 캐시 우선, 없으면 Yahoo Finance에서 수집.

    Args:
        ticker: 종목 코드 (예: "005930", "NVDA")
        market: 시장 (예: "KRX", "NASDAQ")
        start: 시작일 "YYYY-MM-DD"
        end: 종료일 "YYYY-MM-DD"
        use_cache: 캐시 사용 여부
        cache_dir: 캐시 디렉토리
    """
    yf_ticker = normalize_ticker(ticker, market)

    if use_cache:
        cached = load_ohlcv_from_cache(ticker, cache_dir)
        if cached is not None:
            cached_start = str(cached.index.min().date())
            cached_end = str(cached.index.max().date())
            if cached_start <= start and cached_end >= end:
                mask = (cached.index >= start) & (cached.index <= end)
                return cached[mask]

    ticker_obj = yf.Ticker(yf_ticker)
    df = ticker_obj.history(start=start, end=end)

    if df.empty:
        return None

    df = df[["Open", "High", "Low", "Close", "Volume"]]

    if use_cache:
        save_ohlcv_to_cache(df, ticker, cache_dir)

    return df


def calculate_indicators(df: pd.DataFrame) -> pd.DataFrame:
    """
    기술적 지표 계산.

    추가되는 컬럼:
    - RSI_14: 14일 RSI
    - volume_ratio: 5일 평균 대비 당일 거래량 비율
    - pct_change: 전일 대비 등락률 (%)
    - cumulative_5d: 5일 누적 상승률 (%)
    """
    df = df.copy()

    # RSI (14일) — ta 라이브러리 사용
    df["RSI_14"] = ta.momentum.RSIIndicator(df["Close"], window=14).rsi()

    # 전일 대비 등락률
    df["pct_change"] = df["Close"].pct_change() * 100

    # 5일 평균 대비 거래량 비율
    df["volume_ratio"] = df["Volume"] / df["Volume"].rolling(5).mean()

    # 5일 누적 수익률
    df["cumulative_5d"] = df["Close"].pct_change(5) * 100

    return df


def get_market_data_around_date(
    ticker: str,
    market: str,
    trade_date: str,
    window_days: int = 5,
) -> Optional[pd.DataFrame]:
    """
    매매일 기준 전후 N일 데이터 조회 및 지표 계산.

    Args:
        ticker: 종목 코드
        market: 시장 구분
        trade_date: 매매 날짜 "YYYY-MM-DD"
        window_days: 전후 영업일 수 (기본 5)

    Returns:
        기술적 지표가 포함된 OHLCV DataFrame
    """
    date_obj = datetime.strptime(trade_date, "%Y-%m-%d")
    start = (date_obj - timedelta(days=window_days * 2)).strftime("%Y-%m-%d")
    end = (date_obj + timedelta(days=window_days * 2)).strftime("%Y-%m-%d")

    df = fetch_ohlcv(ticker, market, start=start, end=end)
    if df is None or df.empty:
        return None

    df = calculate_indicators(df)
    return df


def summarize_on_date(df: pd.DataFrame, trade_date: str) -> dict:
    """
    특정 날짜의 기술적 지표 요약 반환.

    Returns:
        {
            "date": "2024-01-15",
            "close": 70500,
            "rsi": 62.3,
            "volume_ratio": 1.8,
            "pct_change": 1.5,
            "cumulative_5d": 3.2,
            "signal": "특이사항없음" | "과매수(RSI>70)" | ...
        }
    """
    df.index = pd.to_datetime(df.index)
    try:
        row = df.loc[trade_date]
    except KeyError:
        target = pd.Timestamp(trade_date)
        closest_idx = (df.index - target).abs().argmin()
        row = df.iloc[closest_idx]

    rsi = row.get("RSI_14", None)
    volume_ratio = row.get("volume_ratio", None)
    pct_change = row.get("pct_change", None)
    cumulative_5d = row.get("cumulative_5d", None)

    signals = []
    if rsi is not None and not pd.isna(rsi) and rsi > 70:
        signals.append("과매수(RSI>70)")
    if pct_change is not None and not pd.isna(pct_change) and pct_change > 3:
        signals.append("급등추격(+3%↑)")
    if volume_ratio is not None and not pd.isna(volume_ratio) and volume_ratio > 3:
        signals.append("거래량급증(3배↑)")
    signal = ", ".join(signals) if signals else "특이사항없음"

    def safe_round(val, digits):
        if val is None or pd.isna(val):
            return None
        return round(float(val), digits)

    return {
        "date": str(row.name.date()) if hasattr(row.name, "date") else trade_date,
        "close": round(float(row["Close"]), 2),
        "rsi": safe_round(rsi, 1),
        "volume_ratio": safe_round(volume_ratio, 2),
        "pct_change": safe_round(pct_change, 2),
        "cumulative_5d": safe_round(cumulative_5d, 2),
        "signal": signal,
    }


if __name__ == "__main__":
    ensure_cache_dirs()
    df = get_market_data_around_date("005930", "KRX", "2025-04-10")
    if df is not None:
        summary = summarize_on_date(df, "2025-04-10")
        print(summary)
