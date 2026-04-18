"""
새 매매 등록 모드.
사용자에게 대화형으로 입력받아 trades.json에 추가할 항목을 생성한다.
에이전트가 직접 파일을 수정하지 않는다. 사용자가 직접 수정하도록 안내한다.
"""
import json
from datetime import datetime


def prompt_input(label: str, required: bool = True) -> str:
    """사용자 입력 받기"""
    while True:
        value = input(f"{label}: ").strip()
        if value or not required:
            return value
        print("  ※ 필수 입력 항목입니다.")


def generate_trade_id(existing_ids: list[str]) -> str:
    """새 trade ID 생성 (trade_NNN 형식)"""
    nums = []
    for id_ in existing_ids:
        if id_.startswith("trade_"):
            try:
                nums.append(int(id_.split("_")[1]))
            except ValueError:
                pass
    next_num = max(nums) + 1 if nums else 1
    return f"trade_{next_num:03d}"


def collect_trade_input(existing_ids: list[str]) -> dict:
    """대화형 매매 정보 수집"""
    print("\n=== 새 매매 등록 ===\n")

    print("시장을 선택하세요:")
    print("  1. KRX (한국)")
    print("  2. NASDAQ (미국)")
    print("  3. NYSE (미국)")
    market_choice = prompt_input("선택 (1/2/3)")
    market_map = {"1": "KRX", "2": "NASDAQ", "3": "NYSE"}
    market = market_map.get(market_choice, "KRX")

    ticker = prompt_input("종목 코드 (예: 005930, NVDA)")
    name = prompt_input("종목명 (예: 삼성전자, NVIDIA)")

    print("매매 종류:")
    print("  1. 매수 (buy)")
    print("  2. 매도 (sell)")
    action_choice = prompt_input("선택 (1/2)")
    action = "buy" if action_choice == "1" else "sell"

    price_str = prompt_input("체결 가격")
    quantity_str = prompt_input("수량")
    date = prompt_input("날짜 (YYYY-MM-DD, 엔터 = 오늘)", required=False)
    if not date:
        date = datetime.now().strftime("%Y-%m-%d")
    time_str = prompt_input("시간 (HH:MM, 엔터 = 생략)", required=False)
    reason = prompt_input("매수/매도 이유 (근거, 뉴스, 지표 등 자유롭게)")

    if action == "buy":
        status = "holding"
        linked_buy_id = None
        linked_sell_id = None
    else:
        status = "closed"
        linked_buy_id = prompt_input("연결된 매수 ID (없으면 엔터)", required=False) or None
        linked_sell_id = None

    trade_id = generate_trade_id(existing_ids)

    return {
        "id": trade_id,
        "ticker": ticker.upper(),
        "name": name,
        "market": market,
        "action": action,
        "price": float(price_str.replace(",", "")),
        "quantity": int(quantity_str.replace(",", "")),
        "date": date,
        "time": time_str if time_str else None,
        "reason": reason,
        "status": status,
        "linked_sell_id": linked_sell_id,
        "linked_buy_id": linked_buy_id,
    }


def run_add():
    """매매 추가 대화형 실행"""
    from scripts.utils import DATA_PATH, load_trades

    trades = load_trades()
    existing_ids = [t["id"] for t in trades]

    new_trade = collect_trade_input(existing_ids)

    print("\n=== 등록할 매매 내용 ===")
    print(json.dumps(new_trade, ensure_ascii=False, indent=2))

    print(f"""
=== 안내 ===
위 내용을 data/trades.json의 "trades" 배열에 추가해주세요.
파일 경로: {DATA_PATH}

에이전트는 이 파일을 직접 수정하지 않습니다.
""")


if __name__ == "__main__":
    run_add()
