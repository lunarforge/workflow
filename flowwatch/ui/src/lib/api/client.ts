import { createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { WorkflowService } from "../../gen/flowwatch/v1/workflow_service_pb";
import { RunService } from "../../gen/flowwatch/v1/run_service_pb";
import { AnalyticsService } from "../../gen/flowwatch/v1/analytics_service_pb";

const transport = createConnectTransport({
	baseUrl: import.meta.env.VITE_API_URL ?? "http://localhost:8090",
});

export const workflowClient = createClient(WorkflowService, transport);
export const runClient = createClient(RunService, transport);
export const analyticsClient = createClient(AnalyticsService, transport);
