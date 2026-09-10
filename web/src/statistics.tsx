import { useEffect, useState } from "react";
import {
  ArrowUpRight,
  BookOpen,
  Check,
  Clock3,
  TrendingUp,
} from "lucide-react";
import { api, errorText } from "./api";
import { Metric } from "./App";
import { ErrorBox, Spinner } from "./ui";
import type { Statistics } from "./types";

export function StatisticsPage() {
  const [days, setDays] = useState(30);
  const [data, setData] = useState<Statistics | null>(null);
  const [error, setError] = useState("");
  const [revision, setRevision] = useState(0);
  useEffect(() => {
    const controller = new AbortController();
    setError("");
    setData(null);
    const zone = Intl.DateTimeFormat().resolvedOptions().timeZone;
    void api<Statistics>(
      `/statistics?days=${days}&timezone=${encodeURIComponent(zone)}`,
      { signal: controller.signal },
    )
      .then(setData)
      .catch((e) => {
        if (e.name !== "AbortError") setError(errorText(e));
      });
    return () => controller.abort();
  }, [days, revision]);
  return (
    <>
      <div className="page-heading">
        <div>
          <span className="eyebrow">SMALL STEPS ADD UP</span>
          <h1>
            Your progress<span className="heading-dot">.</span>
          </h1>
          <p>A closer look at the knowledge you’re building.</p>
        </div>
        <select
          className="period-select"
          aria-label="Statistics period"
          value={days}
          onChange={(e) => setDays(Number(e.target.value))}
        >
          <option value={7}>Last 7 days</option>
          <option value={30}>Last 30 days</option>
          <option value={90}>Last 90 days</option>
        </select>
      </div>
      {error ? (
        <ErrorBox error={error} retry={() => setRevision((n) => n + 1)} />
      ) : !data ? (
        <Spinner label="Gathering your progress…" />
      ) : (
        <>
          <div className="library-metrics stats-metrics">
            <Metric
              label="Answers given"
              value={data.totals.answers}
              icon={<Check size={17} />}
            />
            <Metric
              label="Materials reviewed"
              value={data.totals.materials_reviewed}
              icon={<BookOpen size={17} />}
              accent
            />
            <Metric
              label="Active days"
              value={data.totals.active_days}
              icon={<Clock3 size={17} />}
            />
            <Metric
              label="Stage promotions"
              value={data.totals.stage_promotions}
              icon={<TrendingUp size={17} />}
            />
          </div>
          <section className="chart-panel">
            <div className="section-top">
              <div>
                <span className="eyebrow">SHOWING UP IS PROGRESS</span>
                <h2>Your learning rhythm</h2>
              </div>
              <span className="chart-legend">
                <span className="status-dot" /> Answers per day
              </span>
            </div>
            <div
              className="bar-chart"
              role="img"
              aria-label={`Daily activity: ${data.totals.answers} answers over ${days} days`}
            >
              {data.daily.map((d) => (
                <div
                  className="bar-column"
                  key={d.date}
                  title={`${d.date}: ${d.answers} answers, ${d.correct} correct`}
                >
                  <div
                    className={`chart-bar ${d.answers ? "has-value" : ""}`}
                    style={{
                      height: `${Math.max(2, (d.answers / Math.max(1, ...data.daily.map((x) => x.answers))) * 100)}%`,
                    }}
                  />
                  <span className="sr-only">
                    {d.date}: {d.answers} answers
                  </span>
                </div>
              ))}
            </div>
            <div className="chart-axis">
              <span>{data.daily[0]?.date}</span>
              <span>{data.daily[Math.floor(data.daily.length / 2)]?.date}</span>
              <span>{data.daily[data.daily.length - 1]?.date}</span>
            </div>
            {!data.totals.answers && (
              <p className="chart-empty">
                Your story starts with your first answer. A little practice will
                bring this chart to life.
              </p>
            )}
          </section>
          <div className="stats-bottom">
            <section className="stat-detail-panel">
              <span className="eyebrow">RECALL AT A GLANCE</span>
              <h2>
                {data.totals.answers
                  ? `${Math.round((data.totals.correct / data.totals.answers) * 100)}%`
                  : "—"}
                <span> correct answers</span>
              </h2>
              <div className="recall-track">
                <span
                  style={{
                    width: `${data.totals.answers ? (data.totals.correct / data.totals.answers) * 100 : 0}%`,
                  }}
                />
              </div>
              <div className="recall-legend">
                <span>
                  <span className="status-dot" />
                  {data.totals.correct} correct
                </span>
                <span>
                  <span className="status-dot wrong" />
                  {data.totals.wrong} to revisit
                </span>
              </div>
              <p>Mistakes are part of remembering. Every return helps.</p>
            </section>
            <section className="stat-detail-panel reflection">
              <ArrowUpRight size={26} />
              <h2>Consistency over perfection.</h2>
              <p>
                {data.totals.active_days
                  ? `You made time for your knowledge on ${data.totals.active_days} ${data.totals.active_days === 1 ? "day" : "days"} this period. Keep finding those small moments.`
                  : "You don’t need a perfect streak. Just a moment of curiosity, whenever you can."}
              </p>
              <span className="muted">
                Dates shown in {data.timezone.replaceAll("_", " ")}.
              </span>
            </section>
          </div>
        </>
      )}
    </>
  );
}
