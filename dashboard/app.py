"""
Stock-Ops Streamlit 대시보드.
현재 보유 현황과 수익률을 시각화한다.
"""
import sys
from pathlib import Path

import pandas as pd
import plotly.express as px
import streamlit as st

sys.path.insert(0, str(Path(__file__).parent.parent))

from scripts.review import compute_trade_statistics
from scripts.utils import get_holding_trades, load_profile, load_trades

st.set_page_config(page_title="Stock-Ops Dashboard", layout="wide")
st.title("Stock-Ops — 매매 현황 대시보드")

trades = load_trades()
profile = load_profile()
holding = get_holding_trades(trades)
stats = compute_trade_statistics(trades)

# 상단 요약 지표
col1, col2, col3, col4 = st.columns(4)
with col1:
    st.metric("총 매매", f"{stats['total_trades']}건")
with col2:
    st.metric("현재 보유", f"{stats['holding_count']}종목")
with col3:
    closed = stats["closed_trades"]
    if closed:
        avg_pnl = sum(t["pnl_pct"] for t in closed) / len(closed)
        st.metric("평균 수익률", f"{avg_pnl:+.1f}%")
    else:
        st.metric("평균 수익률", "N/A")
with col4:
    if closed:
        avg_hold = sum(t["hold_days"] for t in closed) / len(closed)
        st.metric("평균 보유기간", f"{avg_hold:.0f}일")
    else:
        st.metric("평균 보유기간", "N/A")

st.divider()

# 현재 보유 종목
st.subheader("현재 보유 종목")
if holding:
    holding_df = pd.DataFrame(holding)[
        ["ticker", "name", "market", "price", "quantity", "date", "reason"]
    ]
    holding_df.columns = ["코드", "종목명", "시장", "매수가", "수량", "매수일", "매수이유"]
    holding_df["평가금액"] = holding_df["매수가"] * holding_df["수량"]
    st.dataframe(holding_df, use_container_width=True)
else:
    st.info("현재 보유 중인 종목이 없습니다.")

# 완료된 매매 성과
st.subheader("완료된 매매 성과")
if stats["closed_trades"]:
    closed_df = pd.DataFrame(stats["closed_trades"])
    closed_df.columns = ["매수ID", "매도ID", "종목", "수익률(%)", "보유기간(일)"]

    fig = px.bar(
        closed_df,
        x="종목",
        y="수익률(%)",
        color="수익률(%)",
        color_continuous_scale=["red", "lightgray", "green"],
        color_continuous_midpoint=0,
        title="종목별 수익률",
    )
    st.plotly_chart(fig, use_container_width=True)
    st.dataframe(closed_df, use_container_width=True)
else:
    st.info("완료된 매매가 없습니다.")

# 시장별 분포
st.subheader("시장별 매수 분포")
if stats["markets"]:
    market_df = pd.DataFrame(
        [{"시장": k, "매수건수": v} for k, v in stats["markets"].items()]
    )
    fig_pie = px.pie(market_df, values="매수건수", names="시장")
    st.plotly_chart(fig_pie, use_container_width=True)

st.divider()

# 매매 이력 전체
st.subheader("매매 이력")
if trades:
    all_df = pd.DataFrame(trades)[
        ["id", "ticker", "name", "action", "price", "quantity", "date", "status"]
    ]
    all_df.columns = ["ID", "코드", "종목명", "매매", "가격", "수량", "날짜", "상태"]
    st.dataframe(all_df, use_container_width=True)

st.caption("⚠️ 이 대시보드는 투자 자문이 아닙니다. 과거 데이터 기반 참고용입니다.")
