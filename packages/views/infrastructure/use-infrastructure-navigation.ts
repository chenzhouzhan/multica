"use client";

import { useMemo } from "react";
import { requestInfrastructureNavigation } from "@multica/core/infrastructure";
import { useNavigation } from "../navigation";

export function useInfrastructureNavigation() {
  const navigation = useNavigation();
  return useMemo(
    () => ({
      ...navigation,
      push: (path: string) =>
        requestInfrastructureNavigation(() => navigation.push(path)),
    }),
    [navigation],
  );
}
