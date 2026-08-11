import { useProjection, type Context, type Plugin } from "@wuu-v2/client-runtime";
import type { PlanValue } from "./shared.js";

function PlanCard({ client, sessionId }: { client: Context; sessionId?: string }) {
  const plan = useProjection<PlanValue>(client, sessionId ?? "", "plan");
  if (!sessionId || !plan) return null;
  return (
    <section className="wuu-plan-card" aria-label="Plan">
      {plan.explanation ? <p>{plan.explanation}</p> : null}
      <ol>
        {plan.steps.map((step) => (
          <li key={step.id} data-status={step.status}>
            <span aria-hidden="true">
              {step.status === "completed" ? "✓" : step.status === "in_progress" ? "●" : "○"}
            </span>
            {step.text}
          </li>
        ))}
      </ol>
    </section>
  );
}

const styles = `
.wuu-plan-card {
  margin: 0 0 8px;
  padding: 10px 12px;
  border: 1px solid var(--hairline);
  border-radius: 12px;
  color: var(--ink-muted);
  background: var(--surface-2);
  font-size: 12px;
}
.wuu-plan-card p { margin: 0 0 6px; }
.wuu-plan-card ol { display: grid; gap: 4px; margin: 0; padding: 0; list-style: none; }
.wuu-plan-card li { display: grid; grid-template-columns: 14px minmax(0, 1fr); gap: 6px; }
.wuu-plan-card li[data-status="completed"] { opacity: 0.62; text-decoration: line-through; }
`;

const planClient: Plugin = function plan(client) {
  client.slots.contribute("composer/above", {
    id: "plan",
    component: PlanCard,
  });
  client.effect(() => {
    if (typeof document === "undefined") return () => {};
    const style = document.createElement("style");
    style.dataset.wuuPluginStyle = "plan";
    style.textContent = styles;
    document.head.append(style);
    return () => style.remove();
  }, "install Plan styles");
};

planClient.inject = ["clientProjections", "slots"];
export default planClient;
