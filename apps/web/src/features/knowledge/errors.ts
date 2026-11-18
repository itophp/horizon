import { ApiError } from "@/api/client/request";

const embeddingErrors = {
  unavailable: "notConfigured",
  ai_embedding_incomplete: "errors.embeddingIncomplete",
  ai_embedding_model_required: "errors.embeddingIncomplete",
  ai_model_protocol_mismatch: "errors.embeddingInvalid",
  ai_embedding_dashscope_invalid: "errors.embeddingInvalid",
  knowledge_embedding_authentication_failed: "errors.embeddingAuthentication",
  knowledge_vector_store_unavailable: "errors.vectorStore",
} as const;

export function knowledgeEmbeddingError(error: unknown) {
  if (error instanceof ApiError && Object.hasOwn(embeddingErrors, error.code)) {
    return embeddingErrors[error.code as keyof typeof embeddingErrors];
  }
}
