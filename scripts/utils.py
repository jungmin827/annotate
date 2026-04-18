import json
from pathlib import Path
from typing import Optional

import yaml

DATA_PATH = Path(__file__).parent.parent / "data" / "trades.json"
PROFILE_PATH = Path(__file__).parent.parent / "config" / "profile.yml"
MARKET_CACHE_PATH = Path(__file__).parent.parent / "market_cache"
REPORTS_PATH = Path(__file__).parent.parent / "reports"
NEWS_CACHE_PATH = Path(__file__).parent.parent / "news_cache"


def load_trades(path: Path = DATA_PATH) -> list[dict]:
    with open(path, "r", encoding="utf-8") as f:
        data = json.load(f)
    return data.get("trades", [])


def load_profile(path: Path = PROFILE_PATH) -> dict:
    with open(path, "r", encoding="utf-8") as f:
        return yaml.safe_load(f)


def get_trade_by_id(trades: list[dict], trade_id: str) -> Optional[dict]:
    for trade in trades:
        if trade["id"] == trade_id:
            return trade
    return None


def get_holding_trades(trades: list[dict]) -> list[dict]:
    return [t for t in trades if t["status"] == "holding"]


def get_buy_trades(trades: list[dict]) -> list[dict]:
    return [t for t in trades if t["action"] == "buy"]


def ensure_cache_dirs():
    for path in [MARKET_CACHE_PATH, REPORTS_PATH, NEWS_CACHE_PATH]:
        path.mkdir(parents=True, exist_ok=True)
