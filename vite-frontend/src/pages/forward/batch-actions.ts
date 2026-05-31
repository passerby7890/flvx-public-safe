import type {
  BatchOperationFailure,
  BatchOperationResult,
  ForwardBatchJobApiData,
} from "@/api/types";

import {
  getForwardBatchJob,
  startForwardBatchJob,
} from "@/api";
import {
  buildBatchFailureMessage,
  extractBatchFailures,
  extractApiErrorMessage,
} from "@/api/error-message";

export interface ForwardBatchActionOutcome {
  toastVariant: "success" | "error";
  toastMessage: string;
  shouldRefresh: boolean;
  resultTitle?: string;
  resultSummary?: string;
  failureDetails?: BatchOperationFailure[];
  progressPercent?: number;
  progressLabel?: string;
  closeDeleteModal?: boolean;
  closeChangeTunnelModal?: boolean;
  resetTargetTunnel?: boolean;
  closeChangeStrategyModal?: boolean;
  resetTargetStrategy?: boolean;
}

export type ForwardBatchProgressHandler = (
  job: ForwardBatchJobApiData,
) => void | Promise<void>;

const normalizeBatchResult = (value: unknown): BatchOperationResult => {
  const raw = (value ?? {}) as Partial<BatchOperationResult>;

  return {
    successCount: Number(raw.successCount ?? 0),
    failCount: Number(raw.failCount ?? 0),
    failures: extractBatchFailures(raw),
  };
};

const buildBatchToast = (
  result: BatchOperationResult,
  successText: string,
  resultTitle: string,
): Pick<
  ForwardBatchActionOutcome,
  | "toastVariant"
  | "toastMessage"
  | "resultTitle"
  | "resultSummary"
  | "failureDetails"
> => {
  if (result.failCount === 0) {
    return {
      toastVariant: "success",
      toastMessage: successText,
      resultTitle,
      resultSummary: successText,
      failureDetails: [],
    };
  }

  return {
    toastVariant: "error",
    toastMessage: buildBatchFailureMessage(
      result,
      `成功 ${result.successCount} 项，失败 ${result.failCount} 项`,
    ),
    resultTitle,
    resultSummary: `成功 ${result.successCount} 项，失败 ${result.failCount} 项`,
    failureDetails: result.failures || [],
  };
};

const BATCH_JOB_POLL_INTERVAL_MS = 1500;

const sleep = (ms: number) =>
  new Promise((resolve) => {
    window.setTimeout(resolve, ms);
  });

const waitForForwardBatchJob = async (
  jobId: string,
  onProgress?: ForwardBatchProgressHandler,
) => {
  while (true) {
    const response = await getForwardBatchJob(jobId);

    if (response.code !== 0 || !response.data) {
      throw new Error(response.msg || "批量任务状态获取失败");
    }

    const job = response.data;
    await onProgress?.(job);

    if (job.status === "completed" || job.status === "failed") {
      return job;
    }

    await sleep(BATCH_JOB_POLL_INTERVAL_MS);
  }
};

export const executeForwardBatchDelete = async (
  ids: number[],
  onProgress?: ForwardBatchProgressHandler,
): Promise<ForwardBatchActionOutcome> => {
  try {
    const startResponse = await startForwardBatchJob({
      action: "delete",
      ids,
    });

    if (startResponse.code !== 0 || !startResponse.data) {
      return {
        toastVariant: "error",
        toastMessage: startResponse.msg || "删除失败",
        shouldRefresh: false,
      };
    }

    const job = await waitForForwardBatchJob(
      startResponse.data.jobId,
      onProgress,
    );
    const summary = normalizeBatchResult(job);

    return {
      ...buildBatchToast(
        summary,
        `成功删除 ${summary.successCount} 项`,
        "批量删除结果",
      ),
      shouldRefresh: true,
      progressPercent: 100,
      progressLabel: `删除完成：成功 ${summary.successCount} 项`,
      closeDeleteModal: true,
    };
  } catch (error) {
    return {
      toastVariant: "error",
      toastMessage: extractApiErrorMessage(error, "删除失败"),
      shouldRefresh: false,
    };
  }
};

export const executeForwardBatchToggleService = async (
  ids: number[],
  enable: boolean,
  onProgress?: ForwardBatchProgressHandler,
): Promise<ForwardBatchActionOutcome> => {
  const fallback = enable ? "启用失败" : "停用失败";

  try {
    const startResponse = await startForwardBatchJob({
      action: enable ? "resume" : "pause",
      ids,
    });

    if (startResponse.code !== 0 || !startResponse.data) {
      return {
        toastVariant: "error",
        toastMessage: startResponse.msg || fallback,
        shouldRefresh: false,
      };
    }

    const job = await waitForForwardBatchJob(
      startResponse.data.jobId,
      onProgress,
    );
    const summary = normalizeBatchResult(job);

    return {
      ...buildBatchToast(
        summary,
        enable
          ? `成功启用 ${summary.successCount} 项`
          : `成功停用 ${summary.successCount} 项`,
        enable ? "批量启用结果" : "批量停用结果",
      ),
      shouldRefresh: true,
      progressPercent: 100,
      progressLabel: `${enable ? "启用" : "停用"}完成：成功 ${summary.successCount} 项`,
    };
  } catch (error) {
    return {
      toastVariant: "error",
      toastMessage: extractApiErrorMessage(error, fallback),
      shouldRefresh: false,
    };
  }
};

export const executeForwardBatchRedeploy = async (
  ids: number[],
  onProgress?: ForwardBatchProgressHandler,
): Promise<ForwardBatchActionOutcome> => {
  try {
    const startResponse = await startForwardBatchJob({
      action: "redeploy",
      ids,
    });

    if (startResponse.code !== 0 || !startResponse.data) {
      return {
        toastVariant: "error",
        toastMessage: startResponse.msg || "同步到节点失败",
        shouldRefresh: false,
      };
    }

    const job = await waitForForwardBatchJob(
      startResponse.data.jobId,
      onProgress,
    );
    const summary = normalizeBatchResult(job);

    return {
      ...buildBatchToast(
        summary,
        `成功同步到节点 ${summary.successCount} 项`,
        "批量同步结果",
      ),
      shouldRefresh: true,
      progressPercent: 100,
      progressLabel: `同步到节点完成：成功 ${summary.successCount} 项`,
    };
  } catch (error) {
    return {
      toastVariant: "error",
      toastMessage: extractApiErrorMessage(error, "同步到节点失败"),
      shouldRefresh: false,
    };
  }
};

export const executeForwardBatchChangeTunnel = async (
  ids: number[],
  targetTunnelId: number,
  onProgress?: ForwardBatchProgressHandler,
): Promise<ForwardBatchActionOutcome> => {
  try {
    const startResponse = await startForwardBatchJob({
      action: "change_tunnel",
      ids,
      targetTunnelId,
    });

    if (startResponse.code !== 0 || !startResponse.data) {
      return {
        toastVariant: "error",
        toastMessage: startResponse.msg || "隧道失败",
        shouldRefresh: false,
      };
    }

    const job = await waitForForwardBatchJob(
      startResponse.data.jobId,
      onProgress,
    );
    const summary = normalizeBatchResult(job);

    return {
      ...buildBatchToast(
        summary,
        `成功换隧道 ${summary.successCount} 项`,
        "批量换隧道结果",
      ),
      shouldRefresh: true,
      progressPercent: 100,
      progressLabel: `批量换隧道完成：成功 ${summary.successCount} 项`,
      closeChangeTunnelModal: true,
      resetTargetTunnel: true,
    };
  } catch (error) {
    return {
      toastVariant: "error",
      toastMessage: extractApiErrorMessage(error, "隧道失败"),
      shouldRefresh: false,
    };
  }
};

export const executeForwardBatchUpdateStrategy = async (
  ids: number[],
  strategy: string,
  onProgress?: ForwardBatchProgressHandler,
): Promise<ForwardBatchActionOutcome> => {
  try {
    const startResponse = await startForwardBatchJob({
      action: "update_strategy",
      ids,
      strategy,
    });

    if (startResponse.code !== 0 || !startResponse.data) {
      return {
        toastVariant: "error",
        toastMessage: startResponse.msg || "批量修改策略失败",
        shouldRefresh: false,
      };
    }

    const job = await waitForForwardBatchJob(
      startResponse.data.jobId,
      onProgress,
    );
    const summary = normalizeBatchResult(job);

    return {
      ...buildBatchToast(
        summary,
        `成功修改 ${summary.successCount} 项策略`,
        "批量修改策略结果",
      ),
      shouldRefresh: true,
      progressPercent: 100,
      progressLabel: `批量修改策略完成：成功 ${summary.successCount} 项`,
      closeChangeStrategyModal: true,
      resetTargetStrategy: true,
    };
  } catch (error) {
    return {
      toastVariant: "error",
      toastMessage: extractApiErrorMessage(error, "批量修改策略失败"),
      shouldRefresh: false,
    };
  }
};
