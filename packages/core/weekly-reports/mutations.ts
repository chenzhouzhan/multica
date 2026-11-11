import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../api";
import { weeklyKeys } from "./queries";
import type { WeeklyContent, WeeklyOverview } from "./schema";

export function useSaveWeeklyReport(wsId: string, week: string) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: ({
      content,
      revision,
    }: {
      content: WeeklyContent;
      revision: number;
    }) => api.saveWeeklyReport(wsId, week, content, revision),
    onSuccess: (report) => {
      client.setQueryData(weeklyKeys.mine(wsId, week), report);
      void client.invalidateQueries({ queryKey: weeklyKeys.week(wsId, week) });
      void client.invalidateQueries({
        queryKey: weeklyKeys.history(wsId, report.userId),
      });
    },
  });
}
export function useSubmitWeeklyReport(wsId: string, week: string) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (revision: number) =>
      api.submitWeeklyReport(wsId, week, revision),
    onSuccess: (report) => {
      client.setQueryData(weeklyKeys.mine(wsId, week), report);
      void client.invalidateQueries({ queryKey: weeklyKeys.all(wsId) });
    },
  });
}
export function useSaveWeeklyTeam(wsId: string, week: string) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: ({
      overview,
      revision,
    }: {
      overview: WeeklyOverview;
      revision: number;
    }) => api.saveWeeklyTeamReport(wsId, week, overview, revision),
    onSuccess: (report) =>
      client.setQueryData(weeklyKeys.team(wsId, week), report),
  });
}
export function useGenerateWeeklyTeam(wsId: string, week: string) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (revision: number) =>
      api.generateWeeklyTeamReport(wsId, week, revision),
    onSuccess: (report) =>
      client.setQueryData(weeklyKeys.team(wsId, week), report),
  });
}
