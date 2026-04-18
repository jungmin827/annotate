"""
Stock-Ops CLI 진입점.

사용법:
    python main.py add
    python main.py analyze trade_001
    python main.py review
    python main.py brief
    python main.py dashboard
    python main.py why trade_001
"""
import sys


def main():
    if len(sys.argv) < 2:
        print_help()
        return

    command = sys.argv[1].lower()

    if command == "add":
        from scripts.add import run_add
        run_add()

    elif command == "analyze":
        if len(sys.argv) < 3:
            print("사용법: python main.py analyze <trade_id>")
            sys.exit(1)
        from scripts.analyze import analyze_trade
        trade_id = sys.argv[2]
        result = analyze_trade(trade_id)
        print(f"\n리포트 저장됨: {result}")
        print_report(result)

    elif command == "review":
        from scripts.review import run_review
        result = run_review()
        print(f"\n리포트 저장됨: {result}")
        print_report(result)

    elif command == "brief":
        from scripts.brief import run_brief
        result = run_brief()
        print(result)

    elif command == "dashboard":
        print("대시보드를 실행합니다...")
        import subprocess
        subprocess.run(["streamlit", "run", "dashboard/app.py"])

    elif command == "why":
        if len(sys.argv) < 3:
            print("사용법: python main.py why <trade_id>")
            sys.exit(1)
        from scripts.why import run_why
        trade_id = sys.argv[2]
        result = run_why(trade_id)
        print(result)

    else:
        print(f"알 수 없는 커맨드: {command}")
        print_help()


def print_help():
    print("""
Stock-Ops — 매매 기록 분석 파트너

커맨드:
  add                  새 매매 등록 (대화형)
  analyze <trade_id>   특정 매매 단건 분석
  review               전체 매매 이력 패턴 분석
  brief                보유 종목 오늘의 뉴스 브리핑
  dashboard            Streamlit 대시보드 실행
  why <trade_id>       이 매매, 왜 했었는지 되짚기

예시:
  python main.py add
  python main.py analyze trade_001
  python main.py review
""")


def print_report(path: str):
    from pathlib import Path
    content = Path(path).read_text(encoding="utf-8")
    print("\n" + "=" * 60)
    print(content)


if __name__ == "__main__":
    main()
