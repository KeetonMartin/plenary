import { PHASES, phaseIndex } from "../utils";

export function PhaseStepper({ currentPhase }: { currentPhase: string }) {
  const current = phaseIndex(currentPhase);
  return (
    <div className="flex items-center gap-1 w-full">
      {PHASES.map((phase, i) => {
        const isActive = i === current;
        const isComplete = i < current;
        return (
          <div key={phase} className="flex items-center gap-1 flex-1">
            <div
              className={`flex items-center justify-center rounded-full text-xs font-medium h-7 w-7 shrink-0 ${
                isActive
                  ? "bg-primary text-primary-foreground"
                  : isComplete
                  ? "bg-primary/20 text-primary"
                  : "bg-muted text-muted-foreground"
              }`}
            >
              {isComplete ? "\u2713" : i + 1}
            </div>
            <span
              className={`text-xs truncate ${
                isActive ? "font-semibold" : "text-muted-foreground"
              }`}
            >
              {phase.replace("_", " ")}
            </span>
            {i < PHASES.length - 1 && (
              <div
                className={`h-px flex-1 ${
                  isComplete ? "bg-primary/40" : "bg-muted"
                }`}
              />
            )}
          </div>
        );
      })}
    </div>
  );
}
