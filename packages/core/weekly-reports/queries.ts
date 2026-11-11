import { queryOptions } from "@tanstack/react-query";
import { api } from "../api";

export const weeklyKeys = {
  all: (wsId: string) => ["weekly-reports", wsId] as const,
  week: (wsId: string, week: string) =>
    ["weekly-reports", wsId, "week", week] as const,
  mine: (wsId: string, week: string) =>
    ["weekly-reports", wsId, "mine", week] as const,
  history: (wsId: string, userId: string) =>
    ["weekly-reports", wsId, "history", userId] as const,
  team: (wsId: string, week: string) =>
    ["weekly-reports", wsId, "team", week] as const,
};
export const weeklyWeekOptions = (wsId: string, week: string) =>
  queryOptions({
    queryKey: weeklyKeys.week(wsId, week),
    queryFn: () => api.weeklyWeek(wsId, week),
    enabled: !!wsId,
    refetchInterval: 15000,
  });
export const weeklyMineOptions = (wsId: string, week: string) =>
  queryOptions({
    queryKey: weeklyKeys.mine(wsId, week),
    queryFn: () => api.weeklyMine(wsId, week),
    enabled: !!wsId,
    refetchInterval: 15000,
  });
export const weeklyHistoryOptions = (wsId: string, userId: string) =>
  queryOptions({
    queryKey: weeklyKeys.history(wsId, userId),
    queryFn: () => api.weeklyHistory(wsId, userId),
    enabled: !!wsId && !!userId,
  });
export const weeklyTeamOptions = (wsId: string, week: string) =>
  queryOptions({
    queryKey: weeklyKeys.team(wsId, week),
    queryFn: () => api.weeklyTeam(wsId, week),
    enabled: !!wsId,
    refetchInterval: 15000,
  });
