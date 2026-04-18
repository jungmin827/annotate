"""
보유 종목 뉴스 브리핑 모듈.
Yahoo Finance 뉴스를 수집하고 Claude API로 3줄 요약한다.
"""
import json
import os
from datetime import datetime

import anthropic
import yfinance as yf
from dotenv import load_dotenv

from scripts.fetch_market import normalize_ticker
from scripts.utils import (
    NEWS_CACHE_PATH,
    ensure_cache_dirs,
    get_holding_trades,
    load_trades,
)

load_dotenv()


def fetch_news(ticker: str, market: str, max_items: int = 5) -> list[dict]:
    """
    Yahoo Finance에서 뉴스 수집.

    Returns:
        [{"title": str, "link": str, "published": str}]
    """
    yf_ticker = normalize_ticker(ticker, market)
    ticker_obj = yf.Ticker(yf_ticker)

    try:
        news = ticker_obj.news
        if not news:
            return []
        result = []
        for item in news[:max_items]:
            result.append({
                "title": item.get("title", ""),
                "link": item.get("link", ""),
                "published": item.get("providerPublishTime", ""),
            })
        return result
    except Exception:
        return []


def save_news_cache(ticker: str, news: list[dict]):
    """뉴스 데이터를 캐시에 저장"""
    ticker_cache_dir = NEWS_CACHE_PATH / ticker
    ticker_cache_dir.mkdir(parents=True, exist_ok=True)
    date_str = datetime.now().strftime("%Y-%m-%d")
    cache_file = ticker_cache_dir / f"{date_str}.json"
    cache_file.write_text(json.dumps(news, ensure_ascii=False, indent=2), encoding="utf-8")


def summarize_news_with_claude(ticker_name: str, news_items: list[dict]) -> str:
    """Claude API로 뉴스를 3줄 이내로 요약"""
    if not news_items:
        return "최근 뉴스를 찾을 수 없습니다."

    news_text = "\n".join(f"- {item['title']}" for item in news_items)

    client = anthropic.Anthropic(api_key=os.environ["ANTHROPIC_API_KEY"])
    message = client.messages.create(
        model="claude-haiku-4-5-20251001",
        max_tokens=256,
        messages=[{
            "role": "user",
            "content": f"""다음은 {ticker_name}의 최근 뉴스 헤드라인입니다.
투자 추천 없이, 사실 기반으로 3줄 이내로 요약하세요.

{news_text}

형식:
1. [핵심 내용 1]
2. [핵심 내용 2]
3. [핵심 내용 3] (해당 없으면 생략)
""",
        }],
    )
    return message.content[0].text


def run_brief() -> str:
    """보유 종목 뉴스 브리핑 실행"""
    ensure_cache_dirs()

    trades = load_trades()
    holding = get_holding_trades(trades)

    if not holding:
        return "현재 보유 중인 종목이 없습니다."

    seen = set()
    unique_holdings = []
    for t in holding:
        if t["ticker"] not in seen:
            seen.add(t["ticker"])
            unique_holdings.append(t)

    lines = [f"## 보유 종목 뉴스 브리핑 — {datetime.now().strftime('%Y-%m-%d')}\n"]

    for trade in unique_holdings:
        ticker = trade["ticker"]
        name = trade["name"]
        market = trade["market"]

        print(f"[brief] {name} ({ticker}) 뉴스 수집 중...")
        news = fetch_news(ticker, market)
        save_news_cache(ticker, news)

        summary = summarize_news_with_claude(name, news)
        lines.append(f"### {name} ({ticker})\n{summary}\n")

    return "\n".join(lines)


if __name__ == "__main__":
    result = run_brief()
    print(result)
