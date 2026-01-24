import { env } from "node:process";
import { TsRunner } from "synckit";
export function getWorkerOptions() {
    if (env.NODE_ENV === "test") {
        return {
            timeout: 30000,
            tsRunner: TsRunner.OXC
        };
    }
    else {
        return {
            timeout: 30000
        };
    }
}
//# sourceMappingURL=worker.js.map