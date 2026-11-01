import {
  Fragment,
  createContext,
  useContext,
  useEffect,
  type PropsWithChildren,
} from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  identityLogin,
  identityLogout,
  identitySwitchDepartment,
  identityMe,
  getIdentityMeQueryKey,
  type identityMeResponse,
} from "@/api/generated/baseline";
import type { LoginRequestBody, SessionProfile } from "@/api/generated/models";
import { ApiError, setCSRFToken } from "@/api/client/request";
interface AuthValue {
  profile: SessionProfile | undefined;
  loading: boolean;
  error: unknown;
  signIn: (data: LoginRequestBody) => Promise<void>;
  signOut: () => Promise<void>;
  switchDepartment: (departmentId: number) => Promise<void>;
  refresh: () => Promise<void>;
}
const AuthContext = createContext<AuthValue | null>(null);
export function AuthProvider({ children }: PropsWithChildren) {
  const client = useQueryClient();
  // The token is stored here, before the profile is published: every capability
  // call is a POST and needs the nonce, and a reload must not race it.
  const query = useQuery({
    queryKey: getIdentityMeQueryKey(),
    queryFn: async () => {
      const response = await identityMe();
      if (response.status === 200) setCSRFToken(response.data.csrfToken);
      return response as identityMeResponse;
    },
    retry: false,
    staleTime: 60000,
    refetchOnWindowFocus: true,
    refetchInterval: 30000,
  });
  const profile =
    query.data?.status === 200 &&
    !(query.error instanceof ApiError && query.error.status === 401)
      ? query.data.data
      : undefined;
  useEffect(() => {
    setCSRFToken(profile?.csrfToken ?? "");
    void client.invalidateQueries({
      predicate: (item) => item.queryKey[0] !== getIdentityMeQueryKey()[0],
    });
  }, [profile?.csrfToken, client]);
  useEffect(() => {
    const refresh = () => {
      void client.invalidateQueries({ queryKey: getIdentityMeQueryKey() });
    };
    window.addEventListener("baseline:unauthenticated", refresh);
    return () =>
      window.removeEventListener("baseline:unauthenticated", refresh);
  }, [client]);
  const clearPrivateQueries = () =>
    client.removeQueries({
      predicate: (item) => item.queryKey[0] !== getIdentityMeQueryKey()[0],
    });
  const signIn = async (data: LoginRequestBody) => {
    const response = await identityLogin(data);
    if (response.status !== 200)
      throw new ApiError("unauthenticated", response.status);
    clearPrivateQueries();
    setCSRFToken(response.data.profile.csrfToken);
    client.setQueryData<identityMeResponse>(getIdentityMeQueryKey(), {
      data: response.data.profile,
      status: 200,
      headers: response.headers,
    });
  };
  const signOut = async () => {
    window.dispatchEvent(new Event("baseline:webmcp-revoke"));
    await identityLogout();
    clearPrivateQueries();
    setCSRFToken("");
    client.setQueryData<identityMeResponse>(getIdentityMeQueryKey(), {
      data: { code: "unauthenticated" },
      status: 401,
      headers: new Headers(),
    });
  };
  return (
    <AuthContext.Provider
      value={{
        profile,
        loading: query.isPending,
        error: query.error,
        signIn,
        signOut,
        switchDepartment: async (departmentId) => {
          window.dispatchEvent(new Event("baseline:webmcp-revoke"));
          await client.cancelQueries();
          const response = await identitySwitchDepartment({ departmentId });
          if (response.status !== 200)
            throw new ApiError("forbidden", response.status);
          clearPrivateQueries();
          setCSRFToken(response.data.csrfToken);
          client.setQueryData<identityMeResponse>(
            getIdentityMeQueryKey(),
            response,
          );
        },
        refresh: async () => {
          await client.invalidateQueries({ queryKey: getIdentityMeQueryKey() });
        },
      }}
    >
      <Fragment key={profile?.activeDepartmentId}>{children}</Fragment>
    </AuthContext.Provider>
  );
}
export function useAuth() {
  const value = useContext(AuthContext);
  if (!value) throw new Error("AuthProvider is required");
  return value;
}
export function useSession() {
  const { profile } = useAuth();
  if (!profile) throw new Error("A verified session is required");
  return profile;
}
export function usePermission(permission: string) {
  return useSession().permissions.includes(permission);
}
