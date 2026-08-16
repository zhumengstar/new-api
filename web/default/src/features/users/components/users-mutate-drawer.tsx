import { zodResolver } from "@hookform/resolvers/zod";
import { useQuery } from "@tanstack/react-query";
import { Pencil, X } from "lucide-react";
/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useEffect, useMemo, useState } from "react";
import { useForm } from "react-hook-form";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";

import {
  SideDrawerSection,
  sideDrawerContentClassName,
  sideDrawerFooterClassName,
  sideDrawerFormClassName,
  sideDrawerHeaderClassName,
} from "@/components/drawer-layout";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { Textarea } from "@/components/ui/textarea";
import { useIsRoot } from "@/hooks/use-admin";
import { getCurrencyDisplay, getCurrencyLabel } from "@/lib/currency";
import { formatQuota, parseQuotaFromDollars } from "@/lib/format";
import { cn } from "@/lib/utils";

import {
  createUser,
  updateUser,
  getUser,
  getGroups,
  getPerCallModelPrices,
} from "../api";
import { BINDING_FIELDS, ERROR_MESSAGES, SUCCESS_MESSAGES } from "../constants";
import {
  userFormSchema,
  USER_FORM_DEFAULT_VALUES,
  transformFormDataToPayload,
  transformUserToFormDefaults,
} from "../lib";
import type { UserFormValues } from "../lib/user-form";
import type { User } from "../types";
import { UserQuotaDialog } from "./user-quota-dialog";
import { useUsers } from "./users-provider";

const parseUserModelPrices = (setting?: string): Record<string, number> => {
  if (!setting) return {};
  try {
    const parsed = JSON.parse(setting) as {
      user_model_prices?: Record<string, number>;
    };
    return Object.fromEntries(
      Object.entries(parsed.user_model_prices || {}).filter(
        ([model, price]) =>
          model && Number.isFinite(Number(price)) && Number(price) >= 0,
      ),
    );
  } catch {
    return {};
  }
};

type UsersMutateDrawerProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  currentRow?: User;
};

export function UsersMutateDrawer({
  open,
  onOpenChange,
  currentRow,
}: UsersMutateDrawerProps) {
  const { t } = useTranslation();
  const isRoot = useIsRoot();
  const isUpdate = !!currentRow;
  const { triggerRefresh } = useUsers();
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [quotaDialogOpen, setQuotaDialogOpen] = useState(false);
  const [modelSearch, setModelSearch] = useState("");
  const [selectedPriceModels, setSelectedPriceModels] = useState<string[]>([]);
  const [sharedModelPrice, setSharedModelPrice] = useState("");
  const [userModelPrices, setUserModelPrices] = useState<
    Record<string, number>
  >({});

  // Fetch groups
  const { data: groupsData } = useQuery({
    queryKey: ["groups"],
    queryFn: getGroups,
    staleTime: 5 * 60 * 1000,
  });

  const groups = groupsData?.data || [];

  const { data: perCallModelsData } = useQuery({
    queryKey: ["per-call-model-prices"],
    queryFn: getPerCallModelPrices,
    enabled: open && isUpdate && isRoot,
    staleTime: 60 * 1000,
  });
  const perCallModels = perCallModelsData?.data;
  const filteredPerCallModels = useMemo(() => {
    const keyword = modelSearch.trim().toLowerCase();
    return keyword
      ? (perCallModels || []).filter((item) =>
          item.model.toLowerCase().includes(keyword),
        )
      : perCallModels || [];
  }, [modelSearch, perCallModels]);

  const form = useForm<UserFormValues>({
    resolver: zodResolver(userFormSchema),
    defaultValues: USER_FORM_DEFAULT_VALUES,
  });

  // Load existing data when updating
  useEffect(() => {
    if (open && isUpdate && currentRow) {
      // For update, fetch fresh data
      void getUser(currentRow.id)
        .then((result) => {
          if (result.success && result.data) {
            form.reset(transformUserToFormDefaults(result.data));
            setUserModelPrices(parseUserModelPrices(result.data.setting));
          }
        })
        .catch(() => undefined);
    } else if (open && !isUpdate) {
      // For create, reset to defaults
      form.reset(USER_FORM_DEFAULT_VALUES);
      setUserModelPrices({});
    }
    setModelSearch("");
    setSelectedPriceModels([]);
    setSharedModelPrice("");
  }, [open, isUpdate, currentRow, form]);

  const { meta: currencyMeta } = getCurrencyDisplay();
  const currencyLabel = getCurrencyLabel();
  const tokensOnly = currencyMeta.kind === "tokens";

  const currentQuotaRaw = form.watch("quota_dollars") || 0;

  const applySharedModelPrice = () => {
    const price = Number(sharedModelPrice);
    if (selectedPriceModels.length === 0) {
      toast.error(t("Select at least one per-request model"));
      return;
    }
    if (!Number.isFinite(price) || price < 0) {
      toast.error(t("Model price must be a non-negative number"));
      return;
    }
    setUserModelPrices((current) => ({
      ...current,
      ...Object.fromEntries(selectedPriceModels.map((model) => [model, price])),
    }));
    setSelectedPriceModels([]);
    setSharedModelPrice("");
  };

  const onSubmit = async (data: UserFormValues) => {
    if (!isUpdate) {
      const passwordLength = data.password?.length || 0;
      if (passwordLength < 8 || passwordLength > 20) {
        form.setError("password", {
          type: "manual",
          message: t("Password must be between 8 and 20 characters"),
        });
        return;
      }
    }

    setIsSubmitting(true);
    try {
      const payload = transformFormDataToPayload(data, currentRow?.id);
      if (isUpdate && isRoot && Array.isArray(perCallModels)) {
        const allowedModels = new Set(perCallModels.map((item) => item.model));
        payload.user_model_prices = Object.fromEntries(
          Object.entries(userModelPrices).filter(([model]) =>
            allowedModels.has(model),
          ),
        );
      }
      const result = isUpdate
        ? await updateUser(payload as typeof payload & { id: number })
        : await createUser(payload);

      if (result.success) {
        toast.success(
          isUpdate
            ? t(SUCCESS_MESSAGES.USER_UPDATED)
            : t(SUCCESS_MESSAGES.USER_CREATED),
        );
        onOpenChange(false);
        triggerRefresh();
      } else {
        const fallbackMessage = isUpdate
          ? t(ERROR_MESSAGES.UPDATE_FAILED)
          : t(ERROR_MESSAGES.CREATE_FAILED);
        toast.error(result.message || fallbackMessage);
      }
    } catch {
      toast.error(t(ERROR_MESSAGES.UNEXPECTED));
    } finally {
      setIsSubmitting(false);
    }
  };

  const refreshUserData = async () => {
    if (!currentRow) return;
    const result = await getUser(currentRow.id);
    if (result.success && result.data) {
      form.reset(transformUserToFormDefaults(result.data));
    }
    triggerRefresh();
  };

  return (
    <>
      <Sheet
        open={open}
        onOpenChange={(v) => {
          onOpenChange(v);
          if (!v) {
            form.reset();
          }
        }}
      >
        <SheetContent
          className={sideDrawerContentClassName("sm:max-w-[600px]")}
        >
          <SheetHeader className={sideDrawerHeaderClassName()}>
            <SheetTitle>
              {isUpdate ? t("Update") : t("Create")} {t("User")}
            </SheetTitle>
            <SheetDescription>
              {isUpdate
                ? t("Update the user by providing necessary info.")
                : t("Add a new user by providing necessary info.")}
            </SheetDescription>
          </SheetHeader>
          <Form {...form}>
            <form
              id="user-form"
              onSubmit={form.handleSubmit(onSubmit)}
              className={sideDrawerFormClassName()}
            >
              {/* Basic Information */}
              <SideDrawerSection>
                <h3 className="text-sm font-medium">
                  {t("Basic Information")}
                </h3>

                <FormField
                  control={form.control}
                  name="username"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t("Username")}</FormLabel>
                      <FormControl>
                        <Input
                          {...field}
                          placeholder={t("Enter username")}
                          disabled={isUpdate}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                {isUpdate && isRoot && (
                  <FormField
                    control={form.control}
                    name="contact"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t("WeChat / QQ")}</FormLabel>
                        <FormControl>
                          <Input
                            {...field}
                            placeholder={t(
                              "Automatically detect WeChat or QQ; separate multiple contacts with commas",
                            )}
                            maxLength={130}
                          />
                        </FormControl>
                        <FormDescription>
                          {t("Only visible to super admins")}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                )}

                {!isUpdate && (
                  <FormField
                    control={form.control}
                    name="role"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t("Role")}</FormLabel>
                        <Select
                          items={[
                            { value: "1", label: t("Common User") },
                            { value: "10", label: t("Admin") },
                          ]}
                          onValueChange={(value) =>
                            value !== null &&
                            field.onChange(Number.parseInt(value))
                          }
                          value={String(field.value)}
                        >
                          <FormControl>
                            <SelectTrigger>
                              <SelectValue placeholder={t("Select a role")} />
                            </SelectTrigger>
                          </FormControl>
                          <SelectContent alignItemWithTrigger={false}>
                            <SelectGroup>
                              <SelectItem value="1">
                                {t("Common User")}
                              </SelectItem>
                              <SelectItem value="10">{t("Admin")}</SelectItem>
                            </SelectGroup>
                          </SelectContent>
                        </Select>
                        <FormDescription>
                          {t("Set the user's role (cannot be Root)")}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                )}

                <FormField
                  control={form.control}
                  name="display_name"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t("Display Name")}</FormLabel>
                      <FormControl>
                        <Input
                          {...field}
                          placeholder={t("Enter display name")}
                        />
                      </FormControl>
                      <FormDescription>
                        {t("Leave empty to use username")}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name="password"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t("Password")}</FormLabel>
                      <FormControl>
                        <Input
                          {...field}
                          type="password"
                          placeholder={
                            isUpdate
                              ? t("Leave empty to keep unchanged")
                              : t("Enter password (min 8 characters)")
                          }
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </SideDrawerSection>

              {/* Group & Quota Settings (Update only) */}
              {isUpdate && (
                <SideDrawerSection>
                  <h3 className="text-sm font-medium">{t("Group & Quota")}</h3>

                  <FormField
                    control={form.control}
                    name="group"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t("Group")}</FormLabel>
                        <FormControl>
                          <div className="border-input bg-background rounded-md border p-3">
                            <div className="mb-3 flex min-h-6 flex-wrap gap-1.5">
                              {(field.value || []).length > 0 ? (
                                field.value?.map((group) => (
                                  <Badge
                                    key={group}
                                    variant="secondary"
                                    className="max-w-full"
                                  >
                                    <span className="truncate">{group}</span>
                                  </Badge>
                                ))
                              ) : (
                                <span className="text-muted-foreground text-sm">
                                  {t("Select a group")}
                                </span>
                              )}
                            </div>
                            <div className="grid max-h-48 gap-1 overflow-y-auto pr-1">
                              {groups.map((group) => {
                                const selected = (field.value || []).includes(
                                  group,
                                );
                                return (
                                  <label
                                    key={group}
                                    className={cn(
                                      "hover:bg-muted/50 flex cursor-pointer items-center gap-2 rounded-md px-2 py-2 text-sm transition-colors",
                                      selected && "bg-muted",
                                    )}
                                  >
                                    <Checkbox
                                      checked={selected}
                                      onCheckedChange={(checked) => {
                                        const current = field.value || [];
                                        if (checked === true) {
                                          field.onChange([
                                            ...new Set([...current, group]),
                                          ]);
                                          return;
                                        }
                                        const next = current.filter(
                                          (item) => item !== group,
                                        );
                                        field.onChange(next);
                                      }}
                                    />
                                    <span className="truncate">{group}</span>
                                  </label>
                                );
                              })}
                            </div>
                          </div>
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  {isRoot && (
                    <div className="space-y-3">
                      <div>
                        <Label>{t("Custom per-request model pricing")}</Label>
                        <p className="text-muted-foreground mt-1 text-xs">
                          {t("Only per-request models can be selected")}
                        </p>
                      </div>
                      <Input
                        value={modelSearch}
                        onChange={(event) => setModelSearch(event.target.value)}
                        placeholder={t("Search per-request models")}
                      />
                      <div className="border-input max-h-48 overflow-y-auto rounded-md border p-1">
                        {filteredPerCallModels.map((item) => {
                          const selected = selectedPriceModels.includes(
                            item.model,
                          );
                          return (
                            <label
                              key={item.model}
                              className={cn(
                                "hover:bg-muted/50 flex cursor-pointer items-center gap-2 rounded px-2 py-2 text-sm",
                                selected && "bg-muted",
                              )}
                            >
                              <Checkbox
                                checked={selected}
                                onCheckedChange={(checked) =>
                                  setSelectedPriceModels((current) =>
                                    checked === true
                                      ? [...new Set([...current, item.model])]
                                      : current.filter(
                                          (model) => model !== item.model,
                                        ),
                                  )
                                }
                              />
                              <span className="min-w-0 flex-1 truncate">
                                {item.model}
                              </span>
                              <span className="text-muted-foreground text-xs">
                                ${item.price}/request
                              </span>
                            </label>
                          );
                        })}
                      </div>
                      <div className="flex gap-2">
                        <Input
                          type="number"
                          min="0"
                          step="0.001"
                          value={sharedModelPrice}
                          onChange={(event) =>
                            setSharedModelPrice(event.target.value)
                          }
                          placeholder={t("Shared price per request")}
                        />
                        <Button
                          type="button"
                          onClick={applySharedModelPrice}
                          disabled={
                            selectedPriceModels.length === 0 ||
                            sharedModelPrice === ""
                          }
                        >
                          {t("Apply to selected")}
                        </Button>
                      </div>
                      {Object.keys(userModelPrices).length > 0 && (
                        <div className="max-h-52 space-y-2 overflow-y-auto border-t pt-3">
                          {Object.entries(userModelPrices)
                            .sort(([a], [b]) => a.localeCompare(b))
                            .map(([model, price]) => (
                              <div
                                key={model}
                                className="bg-muted/50 flex items-center gap-2 rounded-md px-2 py-1.5"
                              >
                                <span className="min-w-0 flex-1 truncate text-sm">
                                  {model}
                                </span>
                                <Input
                                  type="number"
                                  min="0"
                                  step="0.001"
                                  value={price}
                                  onChange={(event) => {
                                    const nextPrice = Number(
                                      event.target.value,
                                    );
                                    if (
                                      Number.isFinite(nextPrice) &&
                                      nextPrice >= 0
                                    ) {
                                      setUserModelPrices((current) => ({
                                        ...current,
                                        [model]: nextPrice,
                                      }));
                                    }
                                  }}
                                  className="h-8 w-28"
                                />
                                <Button
                                  type="button"
                                  variant="ghost"
                                  size="icon"
                                  title={t("Remove custom price")}
                                  onClick={() =>
                                    setUserModelPrices((current) => {
                                      const next = { ...current };
                                      delete next[model];
                                      return next;
                                    })
                                  }
                                >
                                  <X className="h-4 w-4" />
                                </Button>
                              </div>
                            ))}
                        </div>
                      )}
                    </div>
                  )}

                  <FormField
                    control={form.control}
                    name="quota_dollars"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>
                          {t(
                            isRoot
                              ? "Remaining Quota ({{currency}})"
                              : "Remaining Virtual Quota ({{currency}})",
                            {
                              currency: currencyLabel,
                            },
                          )}
                        </FormLabel>
                        <div className="flex gap-2">
                          <FormControl>
                            <Input
                              value={
                                tokensOnly
                                  ? String(field.value || 0)
                                  : (field.value || 0).toFixed(6)
                              }
                              readOnly
                              className="flex-1"
                            />
                          </FormControl>
                          <Button
                            type="button"
                            variant="outline"
                            onClick={() => setQuotaDialogOpen(true)}
                          >
                            <Pencil className="mr-1 h-4 w-4" />
                            {isRoot
                              ? t("Adjust Quota")
                              : t("Adjust Virtual Quota")}
                          </Button>
                        </div>
                        <FormDescription>
                          {formatQuota(parseQuotaFromDollars(field.value || 0))}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name="remark"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t("Remark")}</FormLabel>
                        <FormControl>
                          <Textarea
                            {...field}
                            placeholder={t(
                              "Admin notes (only visible to admins)",
                            )}
                            rows={3}
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </SideDrawerSection>
              )}

              {/* Binding Information (Read-only) */}
              {isUpdate && (
                <SideDrawerSection>
                  <h3 className="text-sm font-medium">
                    {t("Binding Information")}
                  </h3>
                  <p className="text-muted-foreground text-xs">
                    {t(
                      "Third-party account bindings (read-only, managed by user in profile settings)",
                    )}
                  </p>

                  <div className="flex flex-col gap-3">
                    {BINDING_FIELDS.map(({ key, label }) => (
                      <div key={key}>
                        <Label className="text-muted-foreground text-xs">
                          {t(label)}
                        </Label>
                        <Input
                          value={
                            (currentRow?.[key as keyof User] as string) || "-"
                          }
                          disabled
                          className="mt-1"
                        />
                      </div>
                    ))}
                  </div>
                </SideDrawerSection>
              )}
            </form>
          </Form>
          <SheetFooter className={sideDrawerFooterClassName()}>
            <SheetClose render={<Button variant="outline" />}>
              {t("Close")}
            </SheetClose>
            <Button form="user-form" type="submit" disabled={isSubmitting}>
              {isSubmitting ? t("Saving...") : t("Save changes")}
            </Button>
          </SheetFooter>
        </SheetContent>
      </Sheet>

      {/* Adjust Quota Dialog */}
      {currentRow && (
        <UserQuotaDialog
          open={quotaDialogOpen}
          onOpenChange={setQuotaDialogOpen}
          userId={currentRow.id}
          currentQuota={parseQuotaFromDollars(currentQuotaRaw || 0)}
          onSuccess={refreshUserData}
        />
      )}
    </>
  );
}
