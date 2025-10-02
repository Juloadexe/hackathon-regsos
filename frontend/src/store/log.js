import { defineStore } from "pinia";
import axios from "axios";

export const Logs = defineStore('logs', {
    state: () => ({
        logsData: [],
        filter_logs: [],
        uniqueModules: [],
        url: 'http://localhost:8080/api',
    }),
    actions: {
        async uploadFile(file) {
            try {
                this.clearLogs();
                const formData = new FormData()
                formData.append('file', file)
                const response = await axios.post(`${this.url}/logs`, formData, {
                    headers: {
                        'Content-Type': 'multipart/form-data'
                    }
                })
                await this.getLogsData();
                return response.data
            } catch (error) {
                console.error('Ошибка загрузки файла:', error)
                throw error
            }
        },
        async getLogsData() {
            try {
                const response = await axios.get(`${this.url}/logs`, {
                    params: {
                        ...this.filter_logs,
                    }
                });
                this.logsData = response.data;
            } catch (error) {
                console.error('Ошибка получения данных логов:', error);
            }
        },
        async getModules() {
            this.uniqueModules = this.logsData.logs.map(log => log.Module);
            this.uniqueModules = [...new Set(this.uniqueModules)];
        },
        async setFiters(filters) {
            this.filter_logs = {
                ...this.filter_logs,
                ...Object.fromEntries(Object.entries(filters).filter(([_, v]) => v != null && v !== ''))
            }
            await this.getLogsData();
        },
        async clearFilters() {
            this.filter_logs = { level: '' };
            await this.getLogsData();
        },
        async clearLogs() {
            try {
                const response = await axios.post(`${this.url}/clear`);
                return response;
            } catch (error) {
                console.error('Ошибка удаления:', error)
            }
        }
    }
})